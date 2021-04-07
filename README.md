# scaleway-k8s-node-coffee

The Coffee Machine for Scaleway Kubernetes Nodes

## Whats is it?

This is a controller that (will) do a lot of different things based on changes in a Kubernetes cluster (especially Kapsule).

## Features

### Reserved IP

This feature allows a set of predefined reserved IP to be used as the nodes IP. Once a new node appears, it will try to assign a free reserved IP out of the given list to the node.
It is controlled by the `RESERVED_IPS_POOL` environment variable, it's a list a already existing reserved IP, separated by a comma. For instance:
```bash
RESERVED_IPS_POOL=51.15.15.15,51.15.15.32
```

A label `reserved-ip: true` will be added to the nodes with a reserved IP.

### IP reverse

This feature allows to set the reverse IP of the reserved IP to a custom one. It will only work if a reserved IP is already set on the node (to use with the Reserved IP feature).
You just need to set the `REVERSE_IP_DOMAIN` to the wanted domain. For instance, `REVERSE_IP_DOMAIN=example.com` will update the reserved IP `51.16.17.18` with the reverse `18-17-16-51.example.com`.

### Database ACLs

This feature allows to update the ACL rules of several DB to allow of all the cluster nodes (adding new ones, and removing old ones). It takes a comma separated list of ids (or regional ids). For instance:
```
DATABASE_IDS=11111111-1111-1111-2111-111111111111,nl-ams-1/11111111-1111-1111-2111-111111111112
```

will update the ACL of the databse with ID `11111111-1111-1111-2111-111111111111` in the region specified by the environment variable `SCW_DEFAULT_REGION` and the database `11111111-1111-1111-2111-111111111112` in the `nl-ams` region.

If your database is in a different project than the cluster nodes, please set the environment variable `NODES_IP_SOURCE` to `kubernetes`.

### Security Group

This feature allows you to update multiple security groups with:
- The Public and Private IPs of all nodes of the cluster
- The Node Ports of the NodePort and LoadBalancer services

However due to several lack of features, the deletion of the rules if best effort for the nodes, and non existent for the services.
It is controlled by the `SECURITY_GROUP_IDS` environment variable. It takes a comma separated list of ids (or zonale ids).

## TODO
- tests
- leader elect ?
- ideas?
- helm/kustomize ?

## Deploying

```bash
kubectl create -f https://raw.githubusercontent.com/Sh4d1/scaleway-k8s-node-coffee/main/deploy.yaml
kubectl create -f https://raw.githubusercontent.com/Sh4d1/scaleway-k8s-node-coffee/main/secret.yaml --edit --namespace scaleway-k8s-node-coffee
kubectl create -f https://raw.githubusercontent.com/Sh4d1/scaleway-k8s-node-coffee/main/configmap.yaml --edit --namespace scaleway-k8s-node-coffee
```

## Contribution

Feel free to submit any issue, feature request or pull request :smile:!
