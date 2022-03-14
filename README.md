# Introduction
First, [RTFM](https://cert-manager.io/docs/configuration/acme/dns01/).

Have you read it? If you haven't go read it. Cuz I'll keep everything short.

This is a dns01 solver for [FreeDNS](https://freedns.afraid.org/).

Pull requests welcome. I'm completely unfamiliar with golang. I did it by looking at
other webhook repos and this is the result.

## Install
```bash
$ cd deploy
$ helm show values freedns-webhook > my-values.yaml
$ edit my-values.yaml
$ helm install -n cert-manager [INSTALLATION_NAME] freedns-webhook/ -f my-values.yaml
```

## ClusterIssuer for Let's encrypt staging
```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-staging
spec:
  acme:
    email: myemail@example.com
    server: https://acme-staging-v02.api.letsencrypt.org/directory
    privateKeySecretRef:
      name: le-staging
    solvers:
    - dns01:
        webhook:
          groupName: acme.freedns.afraid.org
          solverName: freedns-solver
          config:
            secretName: freedns-auth
```

## FreeDNS webhook settings
Normally if you haven't changed anything, the default namespace should be
`cert-manager`. It should be within the same namespace for the webhook when
you do `helm install webhook -n cert-manager`.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: freedns-auth
  namespace: cert-manager
data:
  username: [YOUR_USERNAME_IN_BASE64]
  password: [YOUR_PASSWORD_IN_BASE64]
type: Opaque
```

Additionally, the following names can be customized
* acme.freedns.afraid.org
* freedns-auth