# Introduction
First, [RTFM](https://cert-manager.io/docs/configuration/acme/dns01/).

Have you read it? If you haven't go read it. Cuz I'll keep everything short.

This is a dns01 solver for [FreeDNS](https://freedns.afraid.org/).

Pull requests welcome. I'm now somewhat familiar with golang. You can also look at
other and choose the one that fits your need.

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

Additionally, the following names can be customized
* acme.freedns.afraid.org

### UPDATE
2024-10-30
- Merged from upstream, now works on 1.31 cluster

2024-11-02
- Webhook will now properly logs its actions
- Removed permissions to read secrets from pod for obvious reansons
  - Authentication details are now requested from Helm
  - You should remove the old secret `freedns-auth`. It is now handled by Helm.
