# OpenvCloud CSI driver

This repo implements the CSI spec v1.1.0 for [GIG.tech's OpenvCloud](https://gig.tech)

## Ansible role CSI-Driver

OVC CSI driver can be installed with Ansible role [csi-driver](roles/csi-driver)

Sample playbook `install-csi-driver.yaml`:

``` yaml
- hosts: localhost
  vars:
    server_url: ""   # G8's URL
    account: ""      # account name
    client_jwt: ""   # itsyou.online jwt token
    state: installed # define CSI driver state : ["installed", "uninstalled"]. Default to "installed"
  roles:
    - {role: csi-driver}
```

To run the playbook on your `localhost` execute

``` yaml
ansible-playbook install-csi-driver.yaml
```

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
