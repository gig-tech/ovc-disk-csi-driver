# OpenvCloud CSI example

## Prerequisites

 - G8 with OpenvCloud version >= 2.5.3
 - ssh key
 - IYO jwt authorized for the G8 used.  
    [Click here to see how to get your JWT from your accound ID and secret](https://github.com/gig-tech/terraform-provider-ovc#authentication-with-a-jwt)

## Setup kubernetes cluster on OpenvCloud

- Follow demo repo for kubespray on OpenvCloud [here on the demo-terraform-ansible-kubespray repo](https://github.com/gig-tech/demo-terraform-ansible-kubespray/tree/v0.0.1).
    - Make sure you have your ssh key loaded configured in demo

- SSH into mgmt VM and pass loaded keys: `ssh ansible@<public-ip-of-the-cloudspace> -p 2222 -A`
- SSH into kubernetes master node: `ssh ansible@192.168.103.250` (double check the IP on the G8 portal)
- Too be able to access the kubernetes API, switch to the root user.  
From here there are 2 options to set up the CSI driver demo onto the Kubernetes cluster:
    - Copy the kube config file (`~/.kube/config`) onto your host.  
    You may need to replace the `certificate-authority-data: xxxx` field with `insecure-skip-tls-verify: true`
    - Clone this repo onto the master node

## Apply the example setup on the kubernetes cluster

From the example folder of this repo on the host where you can control the kubernetes cluster with the `kubectl` command:

- Apply namespaces: `kubectl apply -f namespaces`
- Apply secrets by first filling in the data according the names of the files in `secret`. Make sure the file has no appending new line.  
    ```
    echo -n "my_g8_account_name"  > secret/account
    echo -n "my_jwt_token"  > secret/client_jwt
    echo -n "my_g8's_url"  > secret/url
    ```

    Then create the secret:
    ```
    kubectl create secret --namespace ovc-disk-csi generic ovc-disk-csi-driver-secret --from-file=secret
    ```
- Apply driver configs: `kubectl apply -f driver`
- Apply app configs: `kubectl apply -f app`

With `kubectl get po -n demo -o wide` you should now see the demo pod running. You should also see in the G8 portal that the disk is mounted onto the worker VM the pod is running on.

## Deleting setup

```
kubectl delete -f app
kubectl delete -f driver
kubectl delete secret ovc-disk-csi-driver-secret --namespace ovc-disk-csi
kubectl delete -f namespaces
```
