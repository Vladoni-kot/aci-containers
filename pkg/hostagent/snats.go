// Copyright 2019 Cisco Systems, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRATIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Handlers for snat updates.

package hostagent

import (
	"encoding/json"
	"fmt"
	"github.com/Sirupsen/logrus"
	snatglobal "github.com/noironetworks/aci-containers/pkg/snatglobalinfo/apis/aci.snat/v1"
	snatglobalclset "github.com/noironetworks/aci-containers/pkg/snatglobalinfo/clientset/versioned"
	snatlocal "github.com/noironetworks/aci-containers/pkg/snatlocalinfo/apis/aci.snat/v1"
	snatpolicy "github.com/noironetworks/aci-containers/pkg/snatpolicy/apis/aci.snat/v1"
	snatpolicyclset "github.com/noironetworks/aci-containers/pkg/snatpolicy/clientset/versioned"
	"github.com/noironetworks/aci-containers/pkg/util"
	"io/ioutil"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/controller"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

// Filename used to create external service file on host
// example snat-external.service
const SnatService = "snat-external"

type ResourceType int

const (
	SERVICE ResourceType = 1 << iota
	POD
	DEPLOYMENT
	NAMESPACE
	CLUSTER
	INVALID
)

type OpflexPortRange struct {
	Start int `json:"start,omitempty"`
	End   int `json:"end,omitempty"`
}

// This structure is to write the  SnatFile
type OpflexSnatIp struct {
	Uuid          string                   `json:"uuid"`
	InterfaceName string                   `json:"interface-name,omitempty"`
	SnatIp        string                   `json:"snat-ip,omitempty"`
	InterfaceMac  string                   `json:"interface-mac,omitempty"`
	Local         bool                     `json:"local,omitempty"`
	DestIpAddress []string                 `json:"dest,omitempty"`
	PortRange     []OpflexPortRange        `json:"port-range,omitempty"`
	InterfaceVlan uint                     `json:"interface-vlan,omitempty"`
	Zone          uint                     `json:"zone,omitempty"`
	Remote        []OpflexSnatIpRemoteInfo `json:"remote,omitempty"`
}

type OpflexSnatIpRemoteInfo struct {
	NodeIp     string            `json:"snat_ip,omitempty"`
	MacAddress string            `json:"mac,omitempty"`
	PortRange  []OpflexPortRange `json:"port-range,omitempty"`
	Refcount   int               `json:"ref,omitempty"`
}
type OpflexSnatGlobalInfo struct {
	SnatIp         string
	MacAddress     string
	PortRange      []OpflexPortRange
	SnatIpUid      string
	SnatPolicyName string
}

type OpflexSnatLocalInfo struct {
	Snatpolicies map[ResourceType][]string //Each resource can represent multiple entries
	PlcyUuids    []string
	MarkDelete   bool
}

func (agent *HostAgent) initSnatGlobalInformerFromClient(
	snatClient *snatglobalclset.Clientset) {
	agent.initSnatGlobalInformerBase(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return snatClient.AciV1().SnatGlobalInfos(metav1.NamespaceAll).List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return snatClient.AciV1().SnatGlobalInfos(metav1.NamespaceAll).Watch(options)
			},
		})
}

func (agent *HostAgent) initSnatPolicyInformerFromClient(
	snatClient *snatpolicyclset.Clientset) {
	agent.initSnatPolicyInformerBase(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return snatClient.AciV1().SnatPolicies(metav1.NamespaceAll).List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return snatClient.AciV1().SnatPolicies(metav1.NamespaceAll).Watch(options)
			},
		})
}

func getsnat(snatfile string) (string, error) {
	raw, err := ioutil.ReadFile(snatfile)
	if err != nil {
		return "", err
	}
	return string(raw), err
}

func writeSnat(snatfile string, snat *OpflexSnatIp) (bool, error) {
	newdata, err := json.MarshalIndent(snat, "", "  ")
	if err != nil {
		return true, err
	}
	existingdata, err := ioutil.ReadFile(snatfile)
	if err == nil && reflect.DeepEqual(existingdata, newdata) {
		return false, nil
	}

	err = ioutil.WriteFile(snatfile, newdata, 0644)
	return true, err
}

func (agent *HostAgent) FormSnatFilePath(uuid string) string {
	return filepath.Join(agent.config.OpFlexSnatDir, uuid+".snat")
}

func SnatLocalInfoLogger(log *logrus.Logger, snat *snatlocal.SnatLocalInfo) *logrus.Entry {
	return log.WithFields(logrus.Fields{
		"namespace": snat.ObjectMeta.Namespace,
		"name":      snat.ObjectMeta.Name,
		"spec":      snat.Spec,
	})
}

func SnatGlobalInfoLogger(log *logrus.Logger, snat *snatglobal.SnatGlobalInfo) *logrus.Entry {
	return log.WithFields(logrus.Fields{
		"namespace": snat.ObjectMeta.Namespace,
		"name":      snat.ObjectMeta.Name,
		"spec":      snat.Spec,
	})
}

func opflexSnatIpLogger(log *logrus.Logger, snatip *OpflexSnatIp) *logrus.Entry {
	return log.WithFields(logrus.Fields{
		"uuid":           snatip.Uuid,
		"snat_ip":        snatip.SnatIp,
		"mac_address":    snatip.InterfaceMac,
		"port_range":     snatip.PortRange,
		"local":          snatip.Local,
		"interface-name": snatip.InterfaceName,
		"interfcae-vlan": snatip.InterfaceVlan,
		"remote":         snatip.Remote,
	})
}

func (agent *HostAgent) initSnatGlobalInformerBase(listWatch *cache.ListWatch) {
	agent.snatGlobalInformer = cache.NewSharedIndexInformer(
		listWatch,
		&snatglobal.SnatGlobalInfo{},
		controller.NoResyncPeriodFunc(),
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)
	agent.snatGlobalInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			agent.snatGlobalInfoUpdate(obj)
		},
		UpdateFunc: func(_ interface{}, obj interface{}) {
			agent.snatGlobalInfoUpdate(obj)
		},
		DeleteFunc: func(obj interface{}) {
			agent.snatGlobalInfoDelete(obj)
		},
	})
	agent.log.Info("Initializing SnatGlobal Info Informers")
}

func (agent *HostAgent) initSnatPolicyInformerBase(listWatch *cache.ListWatch) {
	agent.snatPolicyInformer = cache.NewSharedIndexInformer(
		listWatch,
		&snatpolicy.SnatPolicy{}, controller.NoResyncPeriodFunc(),
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)
	agent.snatPolicyInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			agent.snatPolicyAdded(obj)
		},
		UpdateFunc: func(oldobj interface{}, newobj interface{}) {
			agent.snatPolicyUpdated(oldobj, newobj)
		},
		DeleteFunc: func(obj interface{}) {
			agent.snatPolicyDeleted(obj)
		},
	})
	agent.log.Info("Initializing Snat Policy Informers")
}

func (agent *HostAgent) snatPolicyAdded(obj interface{}) {
	agent.indexMutex.Lock()
	defer agent.indexMutex.Unlock()
	agent.log.Info("Policy Info Added: ")
	policyinfo := obj.(*snatpolicy.SnatPolicy)
	agent.snatPolicyCache[policyinfo.ObjectMeta.Name] = policyinfo
	agent.handleSnatUpdate(policyinfo)
}

func (agent *HostAgent) snatPolicyUpdated(oldobj interface{}, newobj interface{}) {
	agent.indexMutex.Lock()
	defer agent.indexMutex.Unlock()
	oldpolicyinfo := oldobj.(*snatpolicy.SnatPolicy)
	newpolicyinfo := newobj.(*snatpolicy.SnatPolicy)
	agent.log.Info("Policy Info Updated")
	if reflect.DeepEqual(oldpolicyinfo, newpolicyinfo) {
		return
	}
	agent.snatPolicyCache[newpolicyinfo.ObjectMeta.Name] = newpolicyinfo
	agent.handleSnatUpdate(newpolicyinfo)
}

func (agent *HostAgent) snatPolicyDeleted(obj interface{}) {
	agent.indexMutex.Lock()
	defer agent.indexMutex.Unlock()
	policyinfo := obj.(*snatpolicy.SnatPolicy)
	policyinfokey, err := cache.MetaNamespaceKeyFunc(policyinfo)
	if err != nil {
		return
	}
	agent.log.Debug("Policy Info Deleted: ", policyinfokey)
	delete(agent.snatPolicyCache, policyinfo.ObjectMeta.Name)
	agent.handleSnatUpdate(policyinfo)
}

func (agent *HostAgent) handleSnatUpdate(policy *snatpolicy.SnatPolicy) {
	// First Parse the policy and check for applicability
	// list all the Pods based on labels and namespace
	agent.log.Info("Handle snatUpdate: ", policy)
	_, err := cache.MetaNamespaceKeyFunc(policy)
	if err != nil {
		return
	}
	if _, ok := agent.snatPolicyCache[policy.ObjectMeta.Name]; !ok {
		agent.deletePolicy(policy)
		return
	}
	// set the detination null if there is no destination set
	if len(agent.snatPolicyCache[policy.ObjectMeta.Name].Spec.DestIp) == 0 {
		agent.snatPolicyCache[policy.ObjectMeta.Name].Spec.DestIp = []string{"0.0.0.0/0"}
	}
	// 1.List the targets matching the policy based on policy config
	// 2. find the pods policy is applicable update the pod's policy priority list
	// 3. check the policy is active then Update the NodeInfo with policy.
	uids := make(map[ResourceType][]string)
	switch {
	case len(policy.Spec.SnatIp) == 0:
		//handle policy for service pods
		var services []*v1.Service
		var poduids []string
		selector := labels.SelectorFromSet(labels.Set(policy.Spec.Selector.Labels))
		cache.ListAll(agent.serviceInformer.GetIndexer(), selector,
			func(servobj interface{}) {
				services = append(services, servobj.(*v1.Service))
			})
		// list the pods and apply the policy at service target
		for _, service := range services {
			uids, _ := agent.getPodsMatchingObjet(service, policy.ObjectMeta.Name)
			poduids = append(poduids, uids...)
		}
		uids[SERVICE] = poduids
	case reflect.DeepEqual(policy.Spec.Selector, snatpolicy.PodSelector{}):
		// This Policy will be applied at cluster level
		var poduids []string
		// handle policy for cluster
		for k, _ := range agent.opflexEps {
			poduids = append(poduids, k)
		}
		uids[CLUSTER] = poduids
	case len(policy.Spec.Selector.Labels) == 0:
		// This is namespace based policy
		var poduids []string
		cache.ListAllByNamespace(agent.podInformer.GetIndexer(), policy.Spec.Selector.Namespace, labels.Everything(),
			func(podobj interface{}) {
				pod := podobj.(*v1.Pod)
				if pod.Spec.NodeName == agent.config.NodeName {
					poduids = append(poduids, string(pod.ObjectMeta.UID))
				}
			})
		uids[NAMESPACE] = poduids
	default:
		poduids, deppoduids, nspoduids :=
			agent.getPodUidsMatchingLabel(policy.Spec.Selector.Namespace, policy.Spec.Selector.Labels, policy.ObjectMeta.Name)
		uids[POD] = poduids
		uids[DEPLOYMENT] = deppoduids
		uids[NAMESPACE] = nspoduids
	}
	for res, poduids := range uids {
		agent.applyPolicy(poduids, res, policy.GetName())
	}
}

func (agent *HostAgent) getPodUidsMatchingLabel(namespace string, label map[string]string, policyname string) (poduids []string,
	deppoduids []string, nspoduids []string) {
	// Get all pods matching the label
	// Get all deployments matching the label
	// Get all the namespaces matching the policy label
	selector := labels.SelectorFromSet(labels.Set(label))
	cache.ListAll(agent.podInformer.GetIndexer(), selector,
		func(podobj interface{}) {
			pod := podobj.(*v1.Pod)
			if pod.Spec.NodeName == agent.config.NodeName {
				key, _ := cache.MetaNamespaceKeyFunc(podobj)
				poduids = append(poduids, string(pod.ObjectMeta.UID))
				if _, ok := agent.snatPolicyLabels[key]; ok {
					agent.snatPolicyLabels[key][policyname] = POD
				}
			}
		})
	cache.ListAll(agent.depInformer.GetIndexer(), selector,
		func(depobj interface{}) {
			key, _ := cache.MetaNamespaceKeyFunc(depobj)
			dep := depobj.(*appsv1.Deployment)
			uids, _ := agent.getPodsMatchingObjet(dep, policyname)
			deppoduids = append(deppoduids, uids...)
			if len(deppoduids) > 0 {
				if _, ok := agent.snatPolicyLabels[key]; ok {
					agent.snatPolicyLabels[key][policyname] = DEPLOYMENT
				}
			}
		})
	cache.ListAll(agent.nsInformer.GetIndexer(), selector,
		func(nsobj interface{}) {
			ns := nsobj.(*v1.Namespace)
			key, _ := cache.MetaNamespaceKeyFunc(nsobj)
			uids, _ := agent.getPodsMatchingObjet(ns, policyname)
			nspoduids = append(nspoduids, uids...)
			if len(nspoduids) > 0 {
				if _, ok := agent.snatPolicyLabels[key]; ok {
					agent.snatPolicyLabels[key][policyname] = NAMESPACE
				}
			}
		})
	return
}

func (agent *HostAgent) applyPolicy(poduids []string, res ResourceType, snatPolicyName string) {
	nodeUpdate := false
	if len(poduids) == 0 {
		return
	}
	if _, ok := agent.snatPods[snatPolicyName]; !ok {
		agent.snatPods[snatPolicyName] = make(map[string]ResourceType)
		nodeUpdate = true
	}
	for _, uid := range poduids {
		_, ok := agent.opflexSnatLocalInfos[uid]
		if !ok {
			var localinfo OpflexSnatLocalInfo
			localinfo.Snatpolicies = make(map[ResourceType][]string)
			localinfo.Snatpolicies[res] = append(localinfo.Snatpolicies[res], snatPolicyName)
			agent.opflexSnatLocalInfos[uid] = &localinfo
			agent.snatPods[snatPolicyName][uid] |= res
			agent.log.Debug("applypolicy Res: ", agent.snatPods[snatPolicyName][uid])

		} else {
			present := false
			for _, name := range agent.opflexSnatLocalInfos[uid].Snatpolicies[res] {
				if name == snatPolicyName {
					present = true
				}
			}
			if present == false {
				agent.opflexSnatLocalInfos[uid].Snatpolicies[res] =
					append(agent.opflexSnatLocalInfos[uid].Snatpolicies[res], snatPolicyName)
				agent.snatPods[snatPolicyName][uid] |= res
				agent.log.Debug("applypolicy Res: ", agent.snatPods[snatPolicyName][uid])
			}
			// trigger update  the epfile
		}
	}
	if nodeUpdate == true {
		agent.log.Debug("Schedule the node Sync:")
		agent.scheduleSyncNodeInfo()
	} else {
		agent.log.Debug("Calling Updating EpFile : ", poduids)
		agent.updateEpFiles(poduids)
	}
	return
}

func (agent *HostAgent) syncSnatNodeInfo() bool {
	if !agent.syncEnabled {
		return false
	}
	snatPolicyNames := make(map[string]bool)
	agent.indexMutex.Lock()
	for key, val := range agent.snatPods {
		if len(val) > 0 {
			snatPolicyNames[key] = true
		}
	}
	agent.indexMutex.Unlock()
	env := agent.env.(*K8sEnvironment)
	if env == nil {
		return false
	}
	// send nodeupdate as the policy is activeQ
	if agent.InformNodeInfo(env.nodeInfo, snatPolicyNames) == false {
		agent.log.Debug("Failed to update retry: ", snatPolicyNames)
		return true
	}
	agent.log.Debug("Updated Node Info: ", snatPolicyNames)
	return false
}

func (agent *HostAgent) deletePolicy(policy *snatpolicy.SnatPolicy) {
	pods, ok := agent.snatPods[policy.GetName()]
	var poduids []string
	if !ok {
		return
	}
	for uuid, res := range pods {
		agent.deleteSnatLocalInfo(uuid, res, policy.GetName())
		poduids = append(poduids, uuid)
	}
	agent.updateEpFiles(poduids)
	delete(agent.snatPods, policy.GetName())
	agent.log.Info("SnatPolicy deleted update Nodeinfo: ", policy.GetName())
	agent.scheduleSyncNodeInfo()
	return
}

func (agent *HostAgent) deleteSnatLocalInfo(poduid string, res ResourceType, plcyname string) {
	localinfo, ok := agent.opflexSnatLocalInfos[poduid]
	if ok {
		i := uint(0)
		j := uint(0)
		for i < uint(res) {
			i = 1 << j
			j = j + 1
			if i&uint(res) == i {
				length := len(localinfo.Snatpolicies[ResourceType(i)])
				deletedcount := 0
				for k := 0; k < length; k++ {
					l := k - deletedcount
					if plcyname == localinfo.Snatpolicies[ResourceType(i)][l] {
						agent.log.Info("Delete the Policy name: ", plcyname)
						localinfo.Snatpolicies[ResourceType(i)] = append(localinfo.Snatpolicies[ResourceType(i)][:l], localinfo.Snatpolicies[ResourceType(i)][l+1:]...)
						deletedcount++
					}
				}
				agent.log.Info("Opflex agent and localinfo ", agent.opflexSnatLocalInfos[poduid], localinfo)
				if len(localinfo.Snatpolicies[res]) == 0 {
					delete(localinfo.Snatpolicies, res)
				}
				agent.snatPods[plcyname][poduid] &= ^(res) // clear the bit
				agent.log.Info("Res:  ", agent.snatPods[plcyname][poduid])
				if agent.snatPods[plcyname][poduid] == 0 {
					delete(agent.snatPods[plcyname], poduid)
				}
			}
		}
		// remove policy stack.
	}
}

func (agent *HostAgent) snatGlobalInfoUpdate(obj interface{}) {
	agent.indexMutex.Lock()
	defer agent.indexMutex.Unlock()
	snat := obj.(*snatglobal.SnatGlobalInfo)
	key, err := cache.MetaNamespaceKeyFunc(snat)
	if err != nil {
		SnatGlobalInfoLogger(agent.log, snat).
			Error("Could not create key:" + err.Error())
		return
	}
	agent.log.Info("Snat Global Object added/Updated ", snat)
	agent.doUpdateSnatGlobalInfo(key)
}

func (agent *HostAgent) doUpdateSnatGlobalInfo(key string) {
	snatobj, exists, err :=
		agent.snatGlobalInformer.GetStore().GetByKey(key)
	if err != nil {
		agent.log.Error("Could not lookup snat for " +
			key + ": " + err.Error())
		return
	}
	if !exists || snatobj == nil {
		return
	}
	snat := snatobj.(*snatglobal.SnatGlobalInfo)
	logger := SnatGlobalInfoLogger(agent.log, snat)
	agent.snaGlobalInfoChanged(snatobj, logger)
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func (agent *HostAgent) snaGlobalInfoChanged(snatobj interface{}, logger *logrus.Entry) {
	snat := snatobj.(*snatglobal.SnatGlobalInfo)
	syncSnat := false
	updateLocalInfo := false
	if logger == nil {
		logger = agent.log.WithFields(logrus.Fields{})
	}
	logger.Debug("Snat Global info Changed...")
	globalInfo := snat.Spec.GlobalInfos
	// This case is possible when all the pods will be deleted from that node
	if len(globalInfo) < len(agent.opflexSnatGlobalInfos) {
		for nodename := range agent.opflexSnatGlobalInfos {
			if _, ok := globalInfo[nodename]; !ok {
				delete(agent.opflexSnatGlobalInfos, nodename)
				syncSnat = true
			}
		}
	}
	for nodename, val := range globalInfo {
		var newglobalinfos []*OpflexSnatGlobalInfo
		for _, v := range val {
			portrange := make([]OpflexPortRange, 1)
			portrange[0].Start = v.PortRanges[0].Start
			portrange[0].End = v.PortRanges[0].End
			nodeInfo := &OpflexSnatGlobalInfo{
				SnatIp:         v.SnatIp,
				MacAddress:     v.MacAddress,
				PortRange:      portrange,
				SnatIpUid:      v.SnatIpUid,
				SnatPolicyName: v.SnatPolicyName,
			}
			newglobalinfos = append(newglobalinfos, nodeInfo)
		}
		existing, ok := agent.opflexSnatGlobalInfos[nodename]
		if (ok && !reflect.DeepEqual(existing, newglobalinfos)) || !ok {
			agent.opflexSnatGlobalInfos[nodename] = newglobalinfos
			if nodename == agent.config.NodeName {
				updateLocalInfo = true
			}
			syncSnat = true
		}
	}

	snatFileName := SnatService + ".service"
	filePath := filepath.Join(agent.config.OpFlexServiceDir, snatFileName)
	file_exists := fileExists(filePath)
	if len(agent.opflexSnatGlobalInfos) > 0 {
		// if more than one global infos, create snat ext file
		as := &opflexService{
			Uuid:              SnatService,
			DomainPolicySpace: agent.config.AciVrfTenant,
			DomainName:        agent.config.AciVrf,
			ServiceMode:       "loadbalancer",
			ServiceMappings:   make([]opflexServiceMapping, 0),
			InterfaceName:     agent.config.UplinkIface,
			InterfaceVlan:     uint16(agent.config.ServiceVlan),
			ServiceMac:        agent.serviceEp.Mac,
			InterfaceIp:       agent.serviceEp.Ipv4.String(),
		}
		sm := &opflexServiceMapping{
			Conntrack: true,
		}
		as.ServiceMappings = append(as.ServiceMappings, *sm)
		agent.opflexServices[SnatService] = as
		if !file_exists {
			wrote, err := writeAs(filePath, as)
			if err != nil {
				agent.log.Debug("Unable to write snat ext service file")
			} else if wrote {
				agent.log.Debug("Created snat ext service file")
			}

		}
	} else {
		delete(agent.opflexServices, SnatService)
		// delete snat service file if no global infos exist
		if file_exists {
			err := os.Remove(filePath)
			if err != nil {
				agent.log.Debug("Unable to delete snat ext service file")
			} else {
				agent.log.Debug("Deleted snat ext service file")
			}
		}
	}
	if syncSnat {
		agent.scheduleSyncSnats()
	}
	if updateLocalInfo {
		var poduids []string
		for _, v := range agent.opflexSnatGlobalInfos[agent.config.NodeName] {
			for uuid, _ := range agent.snatPods[v.SnatPolicyName] {
				poduids = append(poduids, uuid)
			}
		}
		agent.log.Info("Updating EpFile GlobalInfo Context: ", poduids)
		agent.updateEpFiles(poduids)
	}
}

func (agent *HostAgent) snatGlobalInfoDelete(obj interface{}) {
	agent.log.Debug("Snat Global Info Obj Delete")
	snat := obj.(*snatglobal.SnatGlobalInfo)
	globalInfo := snat.Spec.GlobalInfos
	for nodename := range globalInfo {
		if _, ok := agent.opflexSnatGlobalInfos[nodename]; ok {
			delete(agent.opflexSnatGlobalInfos, nodename)
		}
	}
}

func (agent *HostAgent) syncSnat() bool {
	if !agent.syncEnabled {
		return false
	}
	agent.log.Debug("Syncing snats")
	agent.indexMutex.Lock()
	opflexSnatIps := make(map[string]*OpflexSnatIp)
	remoteinfo := make(map[string][]OpflexSnatIpRemoteInfo)
	// set the remote info for every snatIp
	for nodename, v := range agent.opflexSnatGlobalInfos {
		for _, ginfo := range v {
			if nodename != agent.config.NodeName {
				var remote OpflexSnatIpRemoteInfo
				remote.MacAddress = ginfo.MacAddress
				remote.PortRange = ginfo.PortRange
				remoteinfo[ginfo.SnatIp] = append(remoteinfo[ginfo.SnatIp], remote)
			}
		}
	}
	agent.log.Debug("Remte: ", remoteinfo)
	// set the Opflex Snat IP information
	localportrange := make(map[string][]OpflexPortRange)
	ginfos, ok := agent.opflexSnatGlobalInfos[agent.config.NodeName]

	if ok {
		for _, ginfo := range ginfos {
			localportrange[ginfo.SnatIp] = ginfo.PortRange
		}
	}

	for _, v := range agent.opflexSnatGlobalInfos {
		for _, ginfo := range v {
			var snatinfo OpflexSnatIp
			// set the local portrange
			snatinfo.InterfaceName = agent.config.UplinkIface
			snatinfo.InterfaceVlan = agent.config.ServiceVlan
			snatinfo.Local = false
			if _, ok := localportrange[ginfo.SnatIp]; ok {
				snatinfo.PortRange = localportrange[ginfo.SnatIp]
				// need to sort the order
				if _, ok := agent.snatPolicyCache[ginfo.SnatPolicyName]; ok {
					snatinfo.DestIpAddress = agent.snatPolicyCache[ginfo.SnatPolicyName].Spec.DestIp
				}
				snatinfo.Local = true

			}
			snatinfo.SnatIp = ginfo.SnatIp
			snatinfo.Uuid = ginfo.SnatIpUid
			snatinfo.Zone = agent.config.Zone
			snatinfo.Remote = remoteinfo[ginfo.SnatIp]
			opflexSnatIps[ginfo.SnatIp] = &snatinfo
			agent.log.Debug("Opflex Snat data IP: ", opflexSnatIps[ginfo.SnatIp])
		}
	}
	agent.indexMutex.Unlock()
	files, err := ioutil.ReadDir(agent.config.OpFlexSnatDir)
	if err != nil {
		agent.log.WithFields(
			logrus.Fields{"SnatDir": agent.config.OpFlexSnatDir},
		).Error("Could not read directory " + err.Error())
		return true
	}
	seen := make(map[string]bool)
	for _, f := range files {
		uuid := f.Name()
		if strings.HasSuffix(uuid, ".snat") {
			uuid = uuid[:len(uuid)-5]
		} else {
			continue
		}

		snatfile := filepath.Join(agent.config.OpFlexSnatDir, f.Name())
		logger := agent.log.WithFields(
			logrus.Fields{"Uuid": uuid})
		existing, ok := opflexSnatIps[uuid]
		if ok {
			fmt.Printf("snatfile:%s\n", snatfile)
			wrote, err := writeSnat(snatfile, existing)
			if err != nil {
				opflexSnatIpLogger(agent.log, existing).Error("Error writing snat file: ", err)
			} else if wrote {
				opflexSnatIpLogger(agent.log, existing).Info("Updated snat")
			}
			seen[uuid] = true
		} else {
			logger.Info("Removing snat")
			os.Remove(snatfile)
		}
	}
	for _, snat := range opflexSnatIps {
		if seen[snat.Uuid] {
			continue
		}
		opflexSnatIpLogger(agent.log, snat).Info("Adding Snat")
		snatfile :=
			agent.FormSnatFilePath(snat.Uuid)
		_, err = writeSnat(snatfile, snat)
		if err != nil {
			opflexSnatIpLogger(agent.log, snat).
				Error("Error writing snat file: ", err)
		}
	}
	agent.log.Debug("Finished snat sync")
	return false
}

func (agent *HostAgent) getPodsMatchingObjet(obj interface{}, policyname string) (poduids []string, res ResourceType) {
	switch obj.(type) {
	case *v1.Pod:
		pod, _ := obj.(*v1.Pod)
		if agent.isPolicyNameSpaceMatches(policyname, pod.ObjectMeta.Namespace) {
			poduids = append(poduids, string(pod.ObjectMeta.UID))
			agent.log.Info("Pod uid: ", poduids)
		}
	case *appsv1.Deployment:
		deployment, _ := obj.(*appsv1.Deployment)
		depkey, _ :=
			cache.MetaNamespaceKeyFunc(deployment)
		if agent.isPolicyNameSpaceMatches(policyname, deployment.ObjectMeta.Namespace) {
			for _, podkey := range agent.depPods.GetPodForObj(depkey) {
				podobj, exists, err := agent.podInformer.GetStore().GetByKey(podkey)
				if err != nil {
					agent.log.Error("Could not lookup pod: ", err)
					continue
				}
				if !exists || podobj == nil {
					agent.log.Error("Object doesn't exist yet ", podkey)
					continue
				}
				poduids = append(poduids, string(podobj.(*v1.Pod).ObjectMeta.UID))
			}
			agent.log.Info("Deployment Pod uid: ", poduids)
		}
		res = DEPLOYMENT
	case *v1.Service:
		service, _ := obj.(*v1.Service)
		selector := labels.SelectorFromSet(labels.Set(service.Spec.Selector))
		if agent.isPolicyNameSpaceMatches(policyname, service.ObjectMeta.Namespace) {
			cache.ListAllByNamespace(agent.podInformer.GetIndexer(), service.ObjectMeta.Namespace, selector,
				func(podobj interface{}) {
					pod := podobj.(*v1.Pod)
					if pod.Spec.NodeName == agent.config.NodeName {
						poduids = append(poduids, string(pod.ObjectMeta.UID))
					}
				})
		}
		agent.log.Info("Service Pod uid: ", poduids)
		res = SERVICE
	case *v1.Namespace:
		ns, _ := obj.(*v1.Namespace)
		if agent.isPolicyNameSpaceMatches(policyname, ns.ObjectMeta.Name) {
			cache.ListAllByNamespace(agent.podInformer.GetIndexer(), ns.ObjectMeta.Name, labels.Everything(),
				func(podobj interface{}) {
					pod := podobj.(*v1.Pod)
					if pod.Spec.NodeName == agent.config.NodeName {
						poduids = append(poduids, string(pod.ObjectMeta.UID))
					}
				})
		}
		agent.log.Info("NameSpace: ", poduids)
		res = NAMESPACE
	default:
	}
	return
}

func (agent *HostAgent) updateEpFiles(poduids []string) {
	syncEp := false
	for _, uid := range poduids {
		localinfo, ok := agent.opflexSnatLocalInfos[uid]
		if !ok {
			continue
		}
		agent.log.Info("Local info: ", localinfo)
		var i uint = 1
		var pos uint = 0
		var policystack []string
		for ; i <= uint(CLUSTER); i = 1 << pos {
			pos = pos + 1
			visted := make(map[string]bool)
			policies, ok := localinfo.Snatpolicies[ResourceType(i)]
			var sortedpolicies []string
			if ok {
				for _, name := range policies {
					if _, ok := visted[name]; !ok {
						visted[name] = true
						sortedpolicies = append(sortedpolicies, name)
					} else {
						continue
					}
				}
				sort.Slice(sortedpolicies, func(i, j int) bool { return agent.compare(sortedpolicies[i], sortedpolicies[j]) })
			}
			policystack = append(policystack, sortedpolicies...)
		}
		var uids []string
		for _, name := range policystack {
			for _, val := range agent.opflexSnatGlobalInfos[agent.config.NodeName] {
				if val.SnatPolicyName == name {
					uids = append(uids, val.SnatIpUid)
				}
			}
			if checkforDefaultRoute(agent.snatPolicyCache[name].Spec.DestIp) == true {
				break
			}
		}
		if !reflect.DeepEqual(agent.opflexSnatLocalInfos[uid].PlcyUuids, uids) {
			agent.log.Info("Update EpFile: ", uids)
			agent.opflexSnatLocalInfos[uid].PlcyUuids = uids
			if len(uids) == 0 {
				delete(agent.opflexSnatLocalInfos, uid)
			}
			syncEp = true
		}
	}
	if syncEp {
		agent.scheduleSyncEps()
	}
}

func (agent *HostAgent) compare(plcy1, plcy2 string) bool {
	sort := false
	for _, a := range agent.snatPolicyCache[plcy1].Spec.DestIp {
		ip_temp := net.ParseIP(a)
		if ip_temp != nil && ip_temp.To4() != nil {
			a = a + "/32"
		}
		for _, b := range agent.snatPolicyCache[plcy2].Spec.DestIp {
			ip_temp := net.ParseIP(b)
			if ip_temp != nil && ip_temp.To4() != nil {
				b = b + "/32"
			}
			ipB, _, _ := net.ParseCIDR(b)
			_, ipnetA, _ := net.ParseCIDR(a)
			ipA, _, _ := net.ParseCIDR(a)
			_, ipnetB, _ := net.ParseCIDR(b)
			switch {
			case ipnetA.Contains(ipB):
				sort = false
			case ipnetB.Contains(ipA):
				sort = true
			default:
				sort = true
			}
		}
	}
	return sort
}

func (agent *HostAgent) getMatchingSnatPolicy(obj interface{}) (snatPolicyNames map[string]ResourceType) {
	snatPolicyNames = make(map[string]ResourceType)
	_, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	metadata, err := meta.Accessor(obj)
	if err != nil {
		return
	}
	if metadata.GetDeletionTimestamp() != nil {
		return
	}
	namespace := metadata.GetNamespace()
	label := metadata.GetLabels()
	res := getResourceType(obj)
	for _, item := range agent.snatPolicyCache {
		if reflect.DeepEqual(item.Spec.Selector, snatpolicy.PodSelector{}) {
			snatPolicyNames[item.ObjectMeta.Name] = CLUSTER
		} else if len(item.Spec.Selector.Labels) == 0 && item.Spec.Selector.Namespace == namespace {
			if res == SERVICE {
				if len(item.Spec.SnatIp) == 0 {
					snatPolicyNames[item.ObjectMeta.Name] = SERVICE
				}
			} else {
				if len(item.Spec.SnatIp) > 0 {
					snatPolicyNames[item.ObjectMeta.Name] = NAMESPACE
				}
			}
		} else {
			matches := false
			if (item.Spec.Selector.Namespace != "" && item.Spec.Selector.Namespace == namespace) ||
				(item.Spec.Selector.Namespace == "") {
				if util.MatchLabels(item.Spec.Selector.Labels, label) &&
					len(item.Spec.SnatIp) > 0 {
					snatPolicyNames[item.ObjectMeta.Name] = res
					matches = true
				}
				if res == POD && matches == false {
					if matches == false {
						if len(item.Spec.SnatIp) == 0 {
							var services []*v1.Service
							selector := labels.SelectorFromSet(labels.Set(label))
							cache.ListAll(agent.serviceInformer.GetIndexer(), selector,
								func(servobj interface{}) {
									services = append(services, servobj.(*v1.Service))
								})
							// list the pods and apply the policy at service target
							for _, service := range services {
								if util.MatchLabels(item.Spec.Selector.Labels, service.ObjectMeta.Labels) {
									matches = true
									snatPolicyNames[item.ObjectMeta.Name] = SERVICE
									break
								}

							}
						} else {
							podKey, _ := cache.MetaNamespaceKeyFunc(obj)
							for _, dpkey := range agent.depPods.GetObjForPod(podKey) {
								depobj, exists, err :=
									agent.depInformer.GetStore().GetByKey(dpkey)
								if err != nil {
									agent.log.Error("Could not lookup snat for " +
										dpkey + ": " + err.Error())
									continue
								}
								if !exists || depobj == nil {
									continue
								}
								if util.MatchLabels(item.Spec.Selector.Labels, depobj.(*appsv1.Deployment).ObjectMeta.Labels) {
									snatPolicyNames[item.ObjectMeta.Name] = DEPLOYMENT
									break
								}
							}
							if matches == false {
								nsobj, exists, err := agent.nsInformer.GetStore().GetByKey(namespace)
								if err != nil {
									agent.log.Error("Could not lookup snat for " +
										namespace + ": " + err.Error())
									continue
								}
								if !exists || nsobj == nil {
									continue
								}
								if util.MatchLabels(item.Spec.Selector.Labels, nsobj.(*v1.Namespace).ObjectMeta.Labels) {
									snatPolicyNames[item.ObjectMeta.Name] = NAMESPACE
									break
								}
								// check for namespace match
							}
						}
					}
				}
			}
		}
	}
	return
}

func (agent *HostAgent) handleObjectUpdate(obj interface{}) {
	objKey, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	agent.log.Info("HandleObject Update: ", objKey)
	plcynames, ok := agent.snatPolicyLabels[objKey]
	if !ok {
		agent.snatPolicyLabels[objKey] = make(map[string]ResourceType)
	}
	sync := false
	if len(plcynames) == 0 {
		polcies := agent.getMatchingSnatPolicy(obj)
		agent.log.Info("HandleObject matching policies: ", polcies)
		for name, res := range polcies {
			poduids, _ := agent.getPodsMatchingObjet(obj, name)
			agent.log.Info("HandleObject Update/Matching Pod Uid's: ", poduids)
			if len(agent.snatPolicyCache[name].Spec.Selector.Labels) == 0 {
				agent.applyPolicy(poduids, res, name)
			} else {
				agent.applyPolicy(poduids, res, name)
				agent.snatPolicyLabels[objKey][name] = res
			}
			sync = true
		}

	} else {
		var delpodlist []string
		matchnames := agent.getMatchingSnatPolicy(obj)
		agent.log.Info("HandleObject matching policies: ", matchnames)
		visited := make(map[string]bool)
		for name, res := range plcynames {
			if _, ok := matchnames[name]; !ok {
				poduids, _ := agent.getPodsMatchingObjet(obj, name)
				for _, uid := range poduids {
					agent.deleteSnatLocalInfo(uid, res, name)
				}
				delpodlist = append(delpodlist, poduids...)
				delete(agent.snatPolicyLabels[objKey], name)
			}
			sync = true
			visited[name] = true
		}
		if len(delpodlist) > 0 {
			agent.updateEpFiles(delpodlist)
		}
		for name, res := range matchnames {
			if visited[name] == true {
				continue
			}
			poduids, _ := agent.getPodsMatchingObjet(obj, name)
			agent.applyPolicy(poduids, res, name)
			agent.snatPolicyLabels[objKey][name] = res
			sync = true
		}
	}
	if sync == true {
		agent.scheduleSyncNodeInfo()
	}
}

func (agent *HostAgent) handleObjectDelete(obj interface{}) {
	objKey, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	metadata, err := meta.Accessor(obj)
	if err != nil {
		return
	}
	agent.log.Debug("HandleObject Delete: ", objKey)
	plcynames := agent.getMatchingSnatPolicy(obj)
	var podidlist []string
	for name, res := range plcynames {
		poduids, _ := agent.getPodsMatchingObjet(obj, name)
		agent.log.Debug("Object deleted: ", poduids, metadata.GetNamespace(), name)
		for _, uid := range poduids {
			if getResourceType(obj) == SERVICE {
				agent.log.Debug("Service deleted update the localInfo: ", name)
				agent.deleteSnatLocalInfo(uid, res, name)
			} else {
				delete(agent.opflexSnatLocalInfos, uid)
				delete(agent.snatPods[name], uid)
			}
		}
		podidlist = append(podidlist, poduids...)
	}
	delete(agent.snatPolicyLabels, objKey)
	if len(podidlist) > 0 {
		agent.scheduleSyncNodeInfo()
		if getResourceType(obj) == SERVICE {
			agent.updateEpFiles(podidlist)
		} else {
			agent.scheduleSyncEps()
		}
	}
}

func (agent *HostAgent) isPolicyNameSpaceMatches(policyName string, namespace string) bool {
	policy, ok := agent.snatPolicyCache[policyName]
	if ok {
		if len(policy.Spec.Selector.Namespace) == 0 || (len(policy.Spec.Selector.Namespace) > 0 &&
			policy.Spec.Selector.Namespace == namespace) {
			return true
		}
	}
	return false
}

func (agent *HostAgent) getSnatUuids(poduuid string) []string {
	agent.indexMutex.Lock()
	val, check := agent.opflexSnatLocalInfos[poduuid]
	agent.indexMutex.Unlock()
	if check {
		agent.log.Debug("Syncing snat uuids: ", val.PlcyUuids)
		return val.PlcyUuids

	} else {
		return []string{}
	}
}

func getResourceType(obj interface{}) ResourceType {
	var res ResourceType
	switch obj.(type) {
	case *v1.Pod:
		res = POD
	case *appsv1.Deployment:
		res = DEPLOYMENT
	case *v1.Service:
		res = SERVICE
	case *v1.Namespace:
		res = NAMESPACE
	default:
	}
	return res
}
func checkforDefaultRoute(destips []string) bool {
	for _, ip := range destips {
		if ip == "0.0.0.0/0" {
			return true
		}
	}
	return false
}
