# FaST-GShare-Autoscaler
FaST-GShare-Autoscaler is a serverless implementation built based on the [FaST-GShare](https://github.com/KontonGu/FaST-GShare.git). This platform introduces a new `FaSTFunc` CRD (Custom Resource Definition) along with its corresponding Operator Controller. FaSTFunc enables FaaS-level control over FaSTPods within FaST-GShare. Users only need to define the container image required for deep model inference and deploy it. FaST-GShare-Autoscaler will automatically scale the model instances based on varying and real-time user workloads, including both vertical and horizontal scaling. Vertical scaling allows for more granular GPU resource allocation, optimizing the sharing and utilization of GPU computational resources.


## Getting Started

### Prerequisites
- go version v1.22.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/fast-gshare-autoscaler:tag
```
ex. 
```sh
make docker-build docker-push IMG=docker.io/kontonpuku666/fast-gshare-autoscaler:test
```

build and update the container in K8S environment
```sh
make docker-clean docker-build docker-push IMG=docker.io/kontonpuku666/fast-gshare-autoscaler:test
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/fast-gshare-autoscaler:tag
```
ex.
```sh
 make deploy IMG=docker.io/kontonpuku666/fast-gshare-autoscaler:test
```
> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create a FaSTFunc**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -f config/samples/sample.yaml
```

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Project Distribution

Following are the steps to build the installer and distribute this project to users.

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/fast-gshare-autoscaler:tag
```

NOTE: The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without
its dependencies.

2. Using the installer

Users can just run kubectl apply -f <URL for YAML BUNDLE> to install the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/fast-gshare-autoscaler/<tag or branch>/dist/install.yaml
```

<!-- ## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html) -->

## License
Copyright 2024 FaST-GShare Authors, KontonGu (**Jianfeng Gu**), et. al.
@Techinical University of Munich, **CAPS Cloud Team**

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

