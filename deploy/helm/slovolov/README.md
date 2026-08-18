# prihorivahori Helm chart

Deploys the Slovolov game server + a DragonflyDB cache (via the
[DragonflyDB Operator](https://www.dragonflydb.io/docs/managing-dragonfly/operator/kubernetes-operator))
+ a CronJob that builds the daily puzzle just after local midnight.

## Prerequisites

- Kubernetes 1.27+ (CronJob `timeZone` field)
- DragonflyDB Operator installed in the cluster (CRD
  `dragonflydb.io/v1alpha1/Dragonfly` must exist)
- Claude API key

Install the operator once per cluster:

```
helm repo add dragonflydb https://dragonflydb.github.io/dragonfly-operator
helm install dragonfly-operator dragonflydb/dragonfly-operator \
  -n dragonfly-operator-system --create-namespace
```

## Install

```
helm install slovolov ./deploy/helm/slovolov \
  --namespace slovolov --create-namespace \
  --set-string anthropic.apiKey=sk-ant-...
```

Or reference an existing Secret:

```
kubectl -n slovolov create secret generic anthropic \
  --from-literal=anthropic-api-key=sk-ant-...

helm install slovolov ./deploy/helm/slovolov \
  --namespace slovolov \
  --set anthropic.existingSecret=anthropic
```

## What gets created

- `Deployment` — game server (2 replicas by default)
- `Service` — ClusterIP :80 → pod :8080
- `Ingress` — optional (`ingress.enabled=true`)
- `Secret` — holds `ANTHROPIC_API_KEY` (unless `anthropic.existingSecret` is set)
- `Dragonfly` — CR consumed by the operator (unless `dragonfly.create=false`)
- `CronJob` × 2 — `<release>-day` and `<release>-week`, each running
  `/app/daily -mode <day|week>` on its own schedule (`daily.*` / `weekly.*`)

## Values

See `values.yaml`. Common overrides:

```
--set image.tag=v0.3.0
--set replicaCount=3
--set ingress.enabled=true --set ingress.host=slovolov.example.com
--set dragonfly.replicas=3
--set puzzle.timezone=Europe/Prague
--set daily.enabled=false
--set weekly.enabled=false
```

Per-job model overrides — empty means the Go client's default (Opus for the
secret word, Sonnet for the bands and for guess scoring):

```
--set anthropic.model=claude-opus-5
--set anthropic.bulkModel=claude-sonnet-5
--set anthropic.guessModel=claude-sonnet-5
```

## Rebuilding a puzzle by hand

The server generates the puzzle on demand if the CronJob has not run, so a
missed run is not fatal. To force a rebuild:

```
kubectl -n slovolov create job --from=cronjob/slovolov-day  day-manual
kubectl -n slovolov create job --from=cronjob/slovolov-week week-manual
```

To replace a puzzle that already exists, or to build a specific one, override
the container args — `/app/daily -mode week -force`, or
`/app/daily -id 2026-08-19`. Regenerating purges the old ranking, the spelling
map, and the cached guess scores for that id first, so no stale scores survive
under the new secret word.
