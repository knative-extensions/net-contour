---
title: Deploying HTTPS services with Contour and cert-manager
layout: page
---

This tutorial shows you how to securely deploy an HTTPS web application on a Kubernetes cluster, using:

- Kubernetes
- Contour, as the Ingress controller
- [JetStack's cert-manager][1] to provision TLS certificates from [the Let's Encrypt project][6]

## Prerequisites

- A Kubernetes cluster deployed in either a data center or a cloud provider with a Kubernetes as a service offering. This tutorial was developed on a GKE cluster running Kubernetes 1.14
- RBAC enabled on your cluster
- Your cluster must be able to request a public IP address from your cloud provider, using a load balancer. If you're on AWS or GKE this is automatic if you deploy a Kubernetes service object of type: LoadBalancer. If you're on your own datacenter you must set it up yourself
- A DNS domain that you control, where you host your web application
- Administrator permissions for all deployment steps

**NOTE:** To use a local cluster like `minikube` or `kind`, see the instructions in [the deployment guide][7].

## Summary

This tutorial walks you through deploying:

1. [Contour][0]
2. [Jetstack cert-manager][1]
3. A sample web application using HTTPProxy

**NOTE:** If you encounter failures related to permissions, make sure the user you are operating as has administrator permissions.

After you've been through the steps the first time, you don't need to repeat deploying Contour and cert-manager for subsequent application deployments. Instead, you can skip to step 3.

## 1. Deploy Contour

Run:

```bash
$ kubectl apply -f {{ site.url }}/quickstart/contour.yaml
```

to set up Contour as a deployment in its own namespace, `projectcontour`, and tell the cloud provider to provision an external IP that is forwarded to the Contour pods.

Check the progress of the deployment with this command:

```
$ kubectl -n projectcontour get po
NAME                      READY     STATUS    RESTARTS   AGE
contour-f9f68994f-kzjdz   2/2       Running   0          6d
contour-f9f68994f-t7h8n   2/2       Running   0          6d
```
After all the `contour` pods reach `Running` status, move on to the next step.

### Access your cluster

Retrieve the external address of the load balancer assigned to Contour's Envoys by your cloud provider:

```
$ kubectl get -n projectcontour service envoy -o wide
NAME      TYPE           CLUSTER-IP     EXTERNAL-IP    PORT(S)                      AGE       SELECTOR
envoy   LoadBalancer   10.51.245.99   35.189.26.87   80:30111/TCP,443:30933/TCP   38d       app=envoy
```

The value of `EXTERNAL-IP` varies by cloud provider. In this example GKE gives a bare IP address; AWS gives you a long DNS name.

To make it easier to work with the external load balancer, the tutorial adds a DNS record to a domain we control that points to this load balancer's IP address:

```
$ host gke.davecheney.com
gke.davecheney.com has address 35.189.26.87
```

On AWS, you specify a `CNAME`, not an `A` record, and it would look something like this:

```
$ host aws.davecheney.com
aws.davecheney.com is an alias for a4d1766f6ce1611e7b27f023b7e83d33–1465548734.ap-southeast-2.elb.amazonaws.com.
a4d1766f6ce1611e7b27f023b7e83d33–1465548734.ap-southeast-2.elb.amazonaws.com has address 52.63.20.117
a4d1766f6ce1611e7b27f023b7e83d33–1465548734.ap-southeast-2.elb.amazonaws.com has address 52.64.233.204
```

In your own data center, you need to arrange for traffic from a public IP address to be forwarded to the cluster IP of the Contour service. This is beyond the scope of the tutorial.

### Testing connectivity

You must deploy at least one Ingress object before Contour can configure Envoy to serve traffic.
Note that as a security feature, Contour does not configure Envoy to expose a port to the internet unless there's a reason it should.
For this tutorial we deploy a version of Kenneth Reitz's [httpbin.org service][3].

To deploy httpbin to your cluster, run this command:

```bash
$ kubectl apply -f {{ site.url }}/examples/httpbin.yaml
```

Check that the pod is running:

```
$ kubectl get po -l app=httpbin
NAME                       READY     STATUS    RESTARTS   AGE
httpbin-67ff6dd458-sfxkb     1/1       Running   0          19d
```

Then type the DNS name you set up in the previous step into a web browser, for example `http://gke.davecheney.com/`. You should see something like:

![httpbin screenshot][8]

You can delete the httpbin service now, or at any time, by running:

```bash
$ kubectl delete -f {{ site.url }}/examples/httpbin.yaml
```

## 2. Deploy jetstack/cert-manager

**NOTE:** cert-manager is a powerful product that provides more functionality than this tutorial demonstrates.
There are plenty of [other ways to deploy cert-manager][4], but they are out of scope.

### Fetch the source manager deployment manifest

To keep things simple, we skip cert-manager's Helm installation, and use the [supplied YAML manifests][5]. Note that the deployment YAMLs don't create the namespace for you, so you must do that first.

Also, the apply command can have `--validate=false` removed for Kubernetes versions > 1.15.
Without this flag, on those versions, the apply command will receive a validation error relating to the CustomResourceDefinition resources. See the [cert-manager deployment guide][4] for more information.

```sh
$ kubectl create namespace cert-manager
$ kubectl apply --validate=false -f https://github.com/jetstack/cert-manager/releases/download/v0.11.0/cert-manager.yaml
```

When cert-manager is up and running you should see something like:

```
$ kubectl -n cert-manager get all
NAME                                           READY   STATUS    RESTARTS   AGE
pod/cert-manager-cainjector-74bb68d67c-8lb2f   1/1     Running   0          40s
pod/cert-manager-f7f8bf74d-65ld9               1/1     Running   0          40s
pod/cert-manager-webhook-645b8bdb7-2h5t6       1/1     Running   0          40s

NAME                           TYPE        CLUSTER-IP     EXTERNAL-IP   PORT(S)    AGE
service/cert-manager           ClusterIP   10.48.13.252   <none>        9402/TCP   40s
service/cert-manager-webhook   ClusterIP   10.48.7.220    <none>        443/TCP    40s

NAME                                      READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/cert-manager              1/1     1            1           40s
deployment.apps/cert-manager-cainjector   1/1     1            1           40s
deployment.apps/cert-manager-webhook      1/1     1            1           40s

NAME                                                 DESIRED   CURRENT   READY   AGE
replicaset.apps/cert-manager-cainjector-74bb68d67c   1         1         1       40s
replicaset.apps/cert-manager-f7f8bf74d               1         1         1       40s
replicaset.apps/cert-manager-webhook-645b8bdb7       1         1         1       40s
```

### Deploy the Let's Encrypt cluster issuer

cert-manager supports two different CRDs for configuration, an `Issuer`, which is scoped to a single namespace,
and a `ClusterIssuer`, which is cluster-wide.

For Contour to be able to serve HTTPS traffic for an Ingress in any namespace, use `ClusterIssuer`.
Create a file called `letsencrypt-staging.yaml` with the following contents:

```yaml
apiVersion: cert-manager.io/v1alpha2
kind: ClusterIssuer
metadata:
  name: letsencrypt-staging
  namespace: cert-manager
spec:
  acme:
    email: user@example.com
    privateKeySecretRef:
      name: letsencrypt-staging
    server: https://acme-staging-v02.api.letsencrypt.org/directory
    solvers:
    - http01:
        ingress:
          class: contour
```

replacing `user@example.com` with your email address.
This is the email address that Let's Encrypt uses to communicate with you about certificates you request.

The staging Let's Encrypt server is not bound by [the API rate limits of the production server][2].
This approach lets you set up and test your environment without worrying about rate limits.
You can then repeat this step for a production Let's Encrypt certificate issuer.

After you edit and save the file, deploy it:

```
$ kubectl apply -f letsencrypt-staging.yaml
clusterissuer "letsencrypt-staging" created
```

You should see several lines in the output of `kubectl -n cert-manager
logs -l app=cert-manager -c cert-manager` informing you that the
`ClusterIssuer` is properly registered:

```
I0220 02:32:50.614141 1 controller.go:138] clusterissuers controller: syncing item 'letsencrypt-staging'
I0220 02:32:52.552107 1 helpers.go:122] Setting lastTransitionTime for ClusterIssuer "letsencrypt-staging" condition "Ready" to 2018–02–20 02:32:52.552092474 +0000 UTC m=+10215.147984505
I0220 02:32:52.560665 1 controller.go:152] clusterissuers controller: Finished processing work item "letsencrypt-staging"
```

## 3. Deploy your first HTTPS site using Ingress

For this tutorial we deploy a version of Kenneth Reitz's [httpbin.org service][3].
We start with the deployment.
Copy the following to a file called `deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: httpbin
  name: httpbin
spec:
  replicas: 1
  selector:
    matchLabels:
      app: httpbin
  strategy:
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 1
    type: RollingUpdate
  template:
    metadata:
      labels:
        app: httpbin
    spec:
      containers:
      - image: docker.io/kennethreitz/httpbin
        name: httpbin
        ports:
        - containerPort: 8080
          name: http
        command: ["gunicorn"]
        args: ["-b", "0.0.0.0:8080", "httpbin:app"]
      dnsPolicy: ClusterFirst
      restartPolicy: Always
      schedulerName: default-scheduler
      securityContext: {}
      terminationGracePeriodSeconds: 30
```

Deploy to your cluster:

```
$ kubectl apply -f deployment.yaml
deployment "httpbin" created
$ kubectl get pod -l app=httpbin
NAME                       READY     STATUS    RESTARTS   AGE
httpbin-67fd96d97c-8j2rr   1/1       Running   0          56m
```

Expose the deployment to the world with a Service. Create a file called `service.yaml` with
the following contents:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: httpbin
spec:
  ports:
  - port: 8080
    protocol: TCP
    targetPort: 8080
  selector:
    app: httpbin
```

and deploy:

```
$ kubectl apply -f service.yaml
service "httpbin" created
$ kubectl get service httpbin
NAME      TYPE        CLUSTER-IP    EXTERNAL-IP   PORT(S)     AGE
httpbin   ClusterIP   10.48.6.155   <none>        8080/TCP   57m
```

Expose the Service to the world with Contour and an Ingress object. Create a file called `ingress.yaml` with
the following contents:

```yaml
apiVersion: networking.k8s.io/v1beta1
kind: Ingress
metadata:
  name: httpbin
spec:
  rules:
  - host: httpbin.davecheney.com
    http:
      paths:
      - backend:
          serviceName: httpbin
          servicePort: 8080
```

The host name, `httpbin.davecheney.com` is a `CNAME` to the `gke.davecheney.com` record that was created in the first section, and must be created in the same place as the `gke.davecheney.com` record was.
That is, in your cloud provider.
This lets requests to `httpbin.davecheney.com` resolve to the external IP address of the Contour service.
They are then forwarded to the Contour pods running in the cluster:

```
$ host httpbin.davecheney.com
httpbin.davecheney.com is an alias for gke.davecheney.com.
gke.davecheney.com has address 35.189.26.87
```

Change the value of `spec.rules.host` to something that you control, and deploy the Ingress to your cluster:

```
$ kubectl apply -f ingress.yaml
ingress "httpbin" created
$ kubectl get ingress httpbin
NAME      HOSTS                    ADDRESS   PORTS     AGE
httpbin   httpbin.davecheney.com             80        58m
```

Now you can type the host name of the service into a browser, or use curl, to verify it's deployed and everything is working:

```
$ curl http://httpbin.davecheney.com/get
{
  "args": {},
  "headers": {
    "Accept": "*/*",
    "Content-Length": "0",
    "Host": "htpbin.davecheney.com",
    "User-Agent": "curl/7.58.0",
    "X-Envoy-Expected-Rq-Timeout-Ms": "15000",
    "X-Envoy-Internal": "true"
  },
  "origin": "10.152.0.2",
  "url": "http://httpbin.davecheney.com/get"
}
```

Excellent, it looks like everything is up and running serving traffic over HTTP.

### Request a TLS certificate from Let's Encrypt

Now it's time to use cert-manager to request a TLS certificate from Let's Encrypt.
Do this by adding some annotations and a `tls:` section to the Ingress spec.

We need to add the following annotations:

- `cert-manager.io/cluster-issuer: letsencrypt-staging`: tells cert-manager to use the `letsencrypt-staging` cluster issuer you just created.
- `kubernetes.io/tls-acme: "true"`: Tells cert-manager to do ACME TLS (what Let's Encrypt uses).
- `ingress.kubernetes.io/force-ssl-redirect: "true"`: tells Contour to redirect HTTP requests to the HTTPS site.
- `kubernetes.io/ingress.class: contour`: Tells Contour that it should handle this Ingress object.

Using `kubectl edit ingress httpbin`:

```yaml
apiVersion: networking.k8s.io/v1beta1
kind: Ingress
metadata:
  name: httpbin
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-staging
    ingress.kubernetes.io/force-ssl-redirect: "true"
    kubernetes.io/ingress.class: contour
    kubernetes.io/tls-acme: "true"
spec:
  tls:
  - secretName: httpbin
    hosts:
    - httpbin.davecheney.com
  rules:
  - host: httpbin.davecheney.com
    http:
      paths:
      - backend:
          serviceName: httpbin
          servicePort: 8080
```

The certificate is issued in the name of the hosts listed in the `tls:` section, `httpbin.davecheney.com` and stored in the secret `httpbin`.
Behind the scenes, cert-manager creates a certificate CRD to manage the lifecycle of the certificate, and then a series of other CRDs to handle the challenge process.

You can watch the progress of the certificate as it's issued:

```
$ kubectl describe certificate httpbin | tail -n 12
Status:
  Conditions:
    Last Transition Time:  2019-11-07T00:37:55Z
    Message:               Waiting for CertificateRequest "httpbinproxy-1925286939" to complete
    Reason:                InProgress
    Status:                False
    Type:                  Ready
Events:
  Type    Reason        Age   From          Message
  ----    ------        ----  ----          -------
  Normal  GeneratedKey  26s   cert-manager  Generated a new private key
  Normal  Requested     26s   cert-manager  Created new CertificateRequest resource "httpbinproxy-1925286939"
```

Wait for the certificate to be issued:

```
$ kubectl describe certificate httpbin | grep -C3 "Certificate is up to date"
Status:
  Conditions:
    Last Transition Time:  2019-11-06T23:47:50Z
    Message:               Certificate is up to date and has not expired
    Reason:                Ready
    Status:                True
    Type:                  Ready
```

A `kubernetes.io/tls` secret is created with the `secretName` specified in the `tls:` field of the Ingress.

```
$ kubectl get secret httpbin
NAME      TYPE                DATA      AGE
httpbin   kubernetes.io/tls   3         3m
```

cert-manager manages the contents of the secret as long as the Ingress is present in your cluster.

You can now visit your site, replacing `http://` with `https://` — and you get a huge security warning!
This is because the certificate was issued by the Let's Encrypt staging servers and has a fake CA.
This is so you can't accidentally use the staging servers to serve real certificates.

```
$ curl https://httpbin.davecheney.com/get
curl: (60) SSL certificate problem: unable to get local issuer certificate
More details here: https://curl.haxx.se/docs/sslcerts.html

curl failed to verify the legitimacy of the server and therefore could not
establish a secure connection to it. To learn more about this situation and
how to fix it, please visit the web page mentioned above.
```

### Switch to Let's Encrypt Production

To request a properly signed certificate from the Let's Encrypt production servers, we create a new `ClusterIssuer`, as before but with some modifications.

Create a file called `letsencrypt-prod.yaml` with the following contents:

```yaml
apiVersion: cert-manager.io/v1alpha2
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
  namespace: cert-manager
spec:
spec:
  acme:
    email: user@example.com
    privateKeySecretRef:
      name: letsencrypt-prod
    server: https://acme-v02.api.letsencrypt.org/directory
    solvers:
    - http01:
        ingress:
          class: contour
```

again replacing `user@example.com` with your email address.

Deploy:

```
$ kubectl apply -f letsencrypt-prod.yaml
clusterissuer "letsencrypt-prod" created
```

Now we use `kubectl edit ingress httpbin` to edit our Ingress to ask for a real certificate from `letsencrypt-prod`:

```yaml
apiVersion: networking.k8s.io/v1beta1
kind: Ingress
metadata:
  name: httpbin
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  ...
```

Next, delete the existing certificate CRD and the Secret that contains the untrusted staging certificate.
This triggers cert-manager to request the certificate again from the Let's Encrypt production servers.

```
$ kubectl delete certificate httpbin
certificate "httpbin" deleted
$ kubectl delete secret httpbin
secret "httpbin" deleted
```

Check that the `httpbin` Secret is recreated, to make sure that the certificate is issued again.
Now revisiting our `https://httpbin.davecheney.com` site should show a valid, trusted, HTTPS certificate.

```
$ curl https://httpbin.davecheney.com/get
{
  "args": {},
  "headers": {
    "Accept": "*/*",
    "Content-Length": "0",
    "Host": "httpbin.davecheney.com",
    "User-Agent": "curl/7.58.0",
    "X-Envoy-Expected-Rq-Timeout-Ms": "15000",
    "X-Envoy-Internal": "true"
  },
  "origin": "10.152.0.2",
  "url": "https://httpbin.davecheney.com/get"
}
```

![httpbin.davecheney.com screenshot][9]

## Making cert-manager work with HTTPProxy

cert-manager currently does not have a way to interact with HTTPProxy objects in order to respond to the HTTP01 challenge correctly.
(See [#950][10] and [#951][11] for details.)
cert-manager does this by creating a new, temporary Ingress object that will direct requests from Let's Encrypt to temporary pods called 'solver pods'.
These pods know how to respond to Let's Encrypt's challenge process for verifying you control the domain you're issuing certificates for.
This means that cert-manager can't be *directly* used for generating certificates for HTTPProxy configuration.

However, we can create a dummy Ingress object that will have cert-manager provision the certificate Secret, so that we can consume it in a HTTPProxy.
This works because Contour expects the Secrets to look the same anyway.

To do this, we need to create our HTTPProxy and Ingress objects.

This example uses the hostname `httpbinproxy.davecheney.com`, remember to create that name before starting.

Firstly, the HTTPProxy:

```yaml
apiVersion: projectcontour.io/v1
kind: HTTPProxy
metadata:
  name: httpbinproxy
spec:
  virtualhost:
    fqdn: httpbinproxy.davecheney.com
    tls:
      secretName: httpbin
  routes:
  - services:
    - name: httpbin
      port: 8080
```

This object will be marked as Invalid by Contour, since the TLS secret doesn't exist yet.
Once that's done, create the dummy Ingress object:

```yaml
apiVersion: networking.k8s.io/v1beta1
kind: Ingress
metadata:
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    ingress.kubernetes.io/force-ssl-redirect: "true"
    kubernetes.io/tls-acme: "true"
  name: httpbinproxy
  namespace: default
spec:
  rules:
  - host: httpbinproxy.davecheney.com
    http:
      paths:
      - backend:
          serviceName: httpbin
          servicePort: 8080
  tls:
  - hosts:
    - httpbinproxy.davecheney.com
    secretName: httpbin
```

Once cert-manager has done its thing, you will have a `httpbinproxy` secret, that will contain the keypair.
Contour will detect that the Secret exists and generate the HTTPProxy config.

After that, you should be able to curl the new site:

```sh
$ curl https://httpbinproxy.davecheney.com/get
{
  "args": {},
  "headers": {
    "Accept": "*/*",
    "Content-Length": "0",
    "Host": "httpbinproxy.davecheney.com",
    "User-Agent": "curl/7.54.0",
    "X-Envoy-Expected-Rq-Timeout-Ms": "15000",
    "X-Envoy-External-Address": "122.106.57.183"
  },
  "origin": "122.106.57.183",
  "url": "https://httpbinproxy.davecheney.com/get"
}
```

### Caveats

This method is a workaround until we can deliver the changes in [#950][10] and [#951][11].

The dummy Ingress record exists only to hold the annotations `ingress-shim` requires.
Because it does not include the `ingress-class: contour` annotation, Contour will not see it and so the configuration of the Ingress does not matter, except that it is valid enough for ingress-shim to use.

## Wrapping up

Now that you've deployed your first HTTPS site using Contour and Let's Encrypt, deploying additional TLS enabled services is much simpler.
Remember that for each HTTPS website you deploy, cert-manager will  create a Certificate CRD that provides the domain name and the name of the target Secret.
The TLS functionality will be enabled when the HTTPProxy contains the `tls:` stanza, and the referenced secret contains a valid keypair.

## Bonus points

For bonus points, it's 2019 and you probably shouldn't be serving traffic over insecure HTTP any more.
Now that TLS is configured for your web service, you can use a feature of Contour to automatically upgrade any HTTP request to the corresponding HTTPS site.

To enable the automatic redirect from HTTP to HTTPS, add this annotation to your Ingress object.

```
metadata:
 annotations:
   ingress.kubernetes.io/force-ssl-redirect: "true"
```
Now any requests to the insecure HTTP version of your site get an unconditional 301 redirect to the HTTPS version:

```
$ curl -v http://httpbin.davecheney.com/get
* Trying 35.189.26.87…
* TCP_NODELAY set
* Connected to httpbin.davecheney.com (35.189.26.87) port 80 (#0)
> GET /get HTTP/1.1
> Host: httpbin.davecheney.com
> User-Agent: curl/7.58.0
> Accept: */*
>
< HTTP/1.1 301 Moved Permanently
< location: https://httpbin.davecheney.com/get
< date: Tue, 20 Feb 2018 04:11:46 GMT
< server: envoy
< content-length: 0
<
* Connection #0 to host httpbin.davecheney.com left intact
```

[0]: {{site.github.repository_url}}
[1]: https://github.com/jetstack/cert-manager
[2]: https://letsencrypt.org/docs/rate-limits/
[3]: http://httpbin.org/
[4]: https://docs.cert-manager.io/en/latest/getting-started/install/kubernetes.html
[5]: https://github.com/jetstack/cert-manager/releases/download/v0.11.0/cert-manager.yaml
[6]: https://letsencrypt.org/getting-started/
[7]: /docs/{{site.latest}}/deploy-options/#get-your-hostname-or-ip-address
[8]: {% link /img/cert-manager/httpbinhomepage.png %}
[9]: {% link /img/cert-manager/httpbin.png %}
[10]: {{site.github.repository_url}}/issues/950
[11]: {{site.github.repository_url}}/issues/951
