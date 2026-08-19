# Jenkins + Harbor + Argo CD delivery model

Production delivery must be split into CI and CD responsibilities.

```text
Developer -> GitHub PR
              |
           Jenkins
              |-- go test / go vet
              |-- Next.js build
              |-- image build
              `-- push immutable SHA tag to Harbor
                         |
                  GitOps repo update
                         |
                     Argo CD
                         |
                    Kubernetes
```

Jenkins should not become the production state manager. After the separate GitOps repository exists, add a Jenkins stage that changes only the image tag/digest in the GitOps repository and commits that desired-state change. Argo CD then reconciles the Kubernetes cluster.

Recommended production image naming:

```text
192.168.100.58/tropical/auth-service:<git-sha>
192.168.100.58/tropical/audit-service:<git-sha>
192.168.100.58/tropical/inventory-service:<git-sha>
192.168.100.58/tropical/sales-service:<git-sha>
192.168.100.58/tropical/dashboard-service:<git-sha>
192.168.100.58/tropical/api-gateway:<git-sha>
192.168.100.58/tropical/web:<git-sha>
```

Vault supplies runtime secrets. Harbor credentials belong in Jenkins credentials, not in source control.
