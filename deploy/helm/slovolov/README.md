# prihorivahori Helm chart

Deploys the Slovolov game server + a DragonflyDB cache (via the
[DragonflyDB Operator](https://www.dragonflydb.io/docs/managing-dragonfly/operator/kubernetes-operator))
+ a one-off Job that warms the embedding cache from the baked-in corpus.

## Prerequisites

- Kubernetes 1.22+
- DragonflyDB Operator installed in the cluster (CRD
  `dragonflydb.io/v1alpha1/Dragonfly` must exist)
- OpenAI API key with billing enabled

Install the operator once per cluster:

```
helm repo add dragonflydb https://dragonflydb.github.io/dragonfly-operator
helm install dragonfly-operator dragonflydb/dragonfly-operator \
  -n dragonfly-operator-system --create-namespace
```

## Install

```
helm install slovolov ./deploy/helm/prihorivahori \
  --namespace slovolov --create-namespace \
  --set-string openai.apiKey=sk-...
```

Or reference an existing Secret:

```
kubectl -n slovolov create secret generic openai \
  --from-literal=openai-api-key=sk-...

helm install slovolov ./deploy/helm/prihorivahori \
  --namespace slovolov \
  --set openai.existingSecret=openai
```

## What gets created

- `Deployment` — game server (2 replicas by default)
- `Service` — ClusterIP :80 → pod :8080
- `Ingress` — optional (`ingress.enabled=true`)
- `Secret` — holds `OPENAI_API_KEY` (unless `openai.existingSecret` is set)
- `Dragonfly` — CR consumed by the operator (unless `dragonfly.create=false`)
- `Job` — post-install/upgrade hook that runs `precompute` against Dragonfly

## Values

See `values.yaml`. Common overrides:

```
--set image.tag=v0.1.0
--set replicaCount=3
--set ingress.enabled=true --set ingress.host=slovolov.example.com
--set dragonfly.replicas=3
--set precompute.enabled=false
```

## Upgrade / rerun precompute

`helm upgrade` re-runs the precompute Job. It skips already-cached words
(idempotent), so re-runs are cheap.
