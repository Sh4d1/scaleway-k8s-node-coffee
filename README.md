# scaleway-k8s-node-coffee

The Coffee Machine for Scaleway Kubernetes Nodes

## Whats is it?

This is a controller that (will) do a lot of different things based on changes in a Kubernetes cluster (especially Kapsule).

## Features

### Reserved IP
### IP reverse
### Database ACLs

## TODO
- tests
- leader elect ?
- ideas?
- helm/kustomize ?

## Deploying

```bash
kubectl create -f https://raw.githubusercontent.com/Sh4d1/scaleway-k8s-node-coffee/main/secret.yaml --edit --namespace scaleway-k8s-node-coffee
kubectl create -f https://raw.githubusercontent.com/Sh4d1/scaleway-k8s-node-coffee/main/configmap.yaml --edit --namespace scaleway-k8s-node-coffee
kubectl create -f https://raw.githubusercontent.com/Sh4d1/scaleway-k8s-node-coffee/main/deploy.yaml
```

## Contribution

Feel free to submit any issue, feature request or pull request :smile:!
