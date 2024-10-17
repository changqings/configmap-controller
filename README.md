## use controller-manager generate, for fast develop controller

## usage:
if configMap has labels `configrestart/deployment=enable`, and configMap.data changed,
controller will restart deployment.spec.volumes which had bind the configMap

## deploy
- make deploy-k8s
