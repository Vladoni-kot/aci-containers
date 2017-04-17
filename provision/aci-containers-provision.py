#!/usr/bin/python

import yaml
import base64
import json
from jinja2 import Environment, PackageLoader
import argparse

def json_indent(s):
    return json.dumps(s, indent=4)

def yaml_quote(s):
    return "'%s'" % str(s).replace("'", "''")

def generate_infra_yaml(config, output):
    env = Environment(
        loader=PackageLoader('aci-containers-provision', 'templates'),
        trim_blocks=True,
        lstrip_blocks=True,
    )
    env.filters['base64enc'] = base64.b64encode
    env.filters['json'] = json_indent
    env.filters['yaml_quote'] = yaml_quote
    template = env.get_template('aci-containers.yaml')
    
    print "Writing kubernetes infrastructure YAML to \"%s\"" % output
    template.stream(config=config).dump(output)

def deep_merge(user, default):
    if isinstance(user,dict) and isinstance(default,dict):
        for k,v in default.iteritems():
            if k not in user:
                user[k] = v
            else:
                user[k] = deep_merge(user[k],v)
    return user
    
if __name__ == "__main__":
    parser = argparse.ArgumentParser(
        description='Provision an ACI containers installation')
    
    parser.add_argument('-c', '--config', required=True,
                        help='A configuration file containing default values')
    parser.add_argument('-o', '--output', required=True,
                        help='Output kubernetes infrastructure YAML to file')
    
    args = parser.parse_args()
    # Default values for configuration
    default_config = {
        "aci_config": {
            "vmm_domain": {
                "pods": ["1"],
                "domain": "kubernetes",
                "controller": "kubernetes",
            },
            "physical_domain": {
                "domain": "physdom",
            }
        },
        "node_config": {
            "encap_type": "vxlan",
            "infra_vlan": 4093,
            "service_vlan": 4003,
        },
        "kubernetes_config": {
            "aci_policy_tenant": "kubernetes",
            "aci_vrf": {
                "tenant": "kubernetes",
                "name": "kubernetes_vrf",
            },
            "aci_l3out": {
                "tenant": "kubernetes",
                "name": "kubernetes_l3out",
                "external_networks" : ["default"],
            },
            "default_endpoint_group": {
                "tenant": "kubernetes",
                "app_profile": "default",
                "group": "default",
            },
            "pod_ip_pool": [
                {"start": "10.1.0.2", "end": "10.1.255.254"}
            ],
            "pod_network": [{
                "subnet": "10.1.0.0/16",
                "gateway": "10.1.0.1",
                "routes": [
                    { "dst": "0.0.0.0/0", "gw": "10.1.0.1" }
                ],
            }],
            "service_ip_pool": [
                {"start": "10.4.1.1", "end": "10.4.255.254"}
            ],
            "static_service_ip_pool": [
                {"start": "10.4.0.1", "end": "10.4.0.255"}
            ],
            "node_service_ip_pool": [
                {"start": "10.6.1.1", "end": "10.6.1.254"}
            ],
        },
        "logging": {
            "controller_log_level": "info",
            "hostagent_log_level": "info",
            "opflexagent_log_level": "info",
            "aim_debug": "False",
        },
        "registry": {
            "image_prefix": "noiro",
        }
    }
    config = default_config

    if args.config:
        print "Loading configuration from \"%s\"" % args.config
        with open(args.config, 'r') as file:
            config = yaml.load(file)
            deep_merge(config, default_config)

    generate_infra_yaml(config, args.output)
