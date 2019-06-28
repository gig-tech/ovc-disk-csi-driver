# OpenvCloud CSI driver

This repo implements the CSI spec v1.1.0 for [GIG.tech's OpenvCloud](https://gig.tech)

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
