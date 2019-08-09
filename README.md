# OpenvCloud CSI driver

This repo implements the CSI spec v1.1.0 for [GIG.tech's OpenvCloud](https://gig.tech)

## Ansible role CSI-Driver

OVC CSI driver can be installed with Ansible role [csi-driver](roles/scr-driver)

Role variables:

``` yaml
# required
server_url: "<G8 url>"

cluster_url: "https://your-g8.gig.tech"
account: "your-account-name"
client_jwt: "ItsyoOnline-JWT-token"
persistent_volume_size: "Size of persistent storage"
```

Example of the configuration file setting credentials and required config see in [config.env.example](config.env.example).

Example playbook:

``` yaml
- hosts: localhost
  vars:
    state: installed
  roles:
    - {role: csi-driver}
```

Variable `state` defines the action to perform. Takes one of two variables: `installed` and `uninstalled`. Default to `installed`.

## Example

This repo includes an example of how to set up a kubernetes cluster on a G8 with CSI driver setup and example deployment which can be found in the [example folder](./example/README.md)
    

## Known issues

- The pod of your application not redeploy to a new node when it's worker node VM is abruptly shutdown as it won't be able to detach the mounted disk. The kubernetes cluster will recover after the worker VM is back up again.

- Not implemented methods of the CSI spec:  
    - GetCapacity
    - ControllerExpandVolume
    - CreateSnapshot
    - DeleteSnapshot
    - ListSnapshots
    - NodeGetVolumeStats
    - NodeExpandVolume
