## Build

You need to go to the `handler` directory and execute the following command:

```shell
docker build -t <tag> .
```

## Example

### Prerequisites

- Dapr, refer to [Getting started with Dapr](https://docs.dapr.io/getting-started/) to install Dapr
- Kubectl, refer to [Install Kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl) to install Kubectl (Also you better have a k8s cluster)

- A Knative runtime function (target function)

  You can refer to [Sample Function Go](https://github.com/OpenFunction/samples/tree/main/functions/Knative/hello-world-go) to create a Knative runtime function.

  Here I assume that the name of this function (Knative Service) is `function-sample-serving-ksvc` . You should be able to see this value with the `kubectl get ksvc` command

- A Kafka server (event source)

  You can refer to [Setting up a Kafka in Kubernetes](https://github.com/dapr/quickstarts/tree/master/bindings#setting-up-a-kafka-in-kubernetes) to deploy a Kafka server.

  Here I assume that the access address of this Kafka server is `dapr-kafka.kafka:9092` .

- A Nats streaming server (event bus)

  You can refer to [Deploy NATS on Kubernetes with Helm Charts](https://nats-io.github.io/k8s/) to deploy a Nats streaming server.

  Here I assume that the access address of this NATS Streaming server is `nats://nats.default:4222` and the cluster ID is `stan` .

### Generate Config

We need to pass the configuration to events-handler in base64 format, you can go into the `example/config` directory, modify the `main.go` and later use the following command to get a base64 formatted string.

```shell
~# go run main.go 
eyJidXNDb21wb25lbnQiOiJ0cmlnZ2VyIiwiYnVzVG9waWMiOlt7Im5hbWUiOiJBIiwibmFtZXNwYWNlIjoiZGVmYXVsdCIsImV2ZW50U291cmNlIjoibXktZXZlbnRzb3VyY2UiLCJldmVudCI6InNhbXBsZS1vbmUifSx7Im5hbWUiOiJCIiwibmFtZXNwYWNlIjoiZGVmYXVsdCIsImV2ZW50U291cmNlIjoibXktZXZlbnRzb3VyY2UiLCJldmVudCI6InNhbXBsZS10d28ifV0sInN1YnNjcmliZXJzIjp7IkEgXHUwMDI2XHUwMDI2IEIiOnsidG9waWMiOiJtZXRyaWNzIn0sIkEgfHwgQiI6eyJzaW5rQ29tcG9uZW50IjoiaHR0cC1zaW5rIn19LCJwb3J0IjoiNTA1MCJ9
```

Go into the `deploy` directory and modify `trigger-handler.yaml`, replacing the contents of `.spec.template.spec.containers.env` with the string obtained above, it is easy to find its exact location.

```yaml
    spec:
      containers:
        - name: trigger
          image: <tag>
          imagePullPolicy: IfNotPresent
          env:
            - name: CONFIG
              value: "eyJidXNDb21wb25lbnQiOiJ0cmlnZ2VyIiwiYnVzVG9waWMiOlt7Im5hbWUiOiJBIiwibmFtZXNwYWNlIjoiZGVmYXVsdCIsImV2ZW50U291cmNlIjoibXktZXZlbnRzb3VyY2UiLCJldmVudCI6InNhbXBsZS1vbmUifSx7Im5hbWUiOiJCIiwibmFtZXNwYWNlIjoiZGVmYXVsdCIsImV2ZW50U291cmNlIjoibXktZXZlbnRzb3VyY2UiLCJldmVudCI6InNhbXBsZS10d28ifV0sInN1YnNjcmliZXJzIjp7IkEgXHUwMDI2XHUwMDI2IEIiOnsidG9waWMiOiJtZXRyaWNzIn0sIkEgfHwgQiI6eyJzaW5rQ29tcG9uZW50IjoiaHR0cC1zaW5rIn19LCJwb3J0IjoiNTA1MCJ9"
```

You also need to update the `sink-component.yaml` in the `example/deploy` directory to update the value of the **url** to the `ksvc` address, like this:

```yaml
apiVersion: dapr.io/v1alpha1
kind: Component
metadata:
  name: http-sink
spec:
  type: bindings.http
  version: v1
  metadata:
    - name: url
      # This is the access address of a Knative Service
      value: http://function-sample-serving-ksvc.default.192.168.0.5.sslip.io
```

### Events Producer

You can either go to [this directory](../events-producer) and customize an event producer, or use the `openfunctiondev/events-producer:latest` image. Once you have determined your selection, update this value to `events-producer.yaml` in the `example/deploy` directory.

### Run

When the above has been prepared, go to the `deploy` directory and execute the following command:

```shell
kubectl apply -f .
```

It can be observed that the target function has been triggered.

```shell
~# kubectl get po
NAME                                                            READY   STATUS    RESTARTS   AGE
events-producer-bf8cd85d7-ghp2v                                 2/2     Running   2          41s
function-sample-serving-ksvc-v100-deployment-5647d775f6-4qmk4   2/2     Running   0          21s
trigger-784cf4cb-rvkrh                                          2/2     Running   0          41s
```

