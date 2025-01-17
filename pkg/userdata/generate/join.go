/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package generate

const workerTempl = `#!talos
version: ""
security: null
services:
  init:
    cni: flannel
  kubeadm:
    configuration: |
      apiVersion: kubeadm.k8s.io/v1beta1
      kind: JoinConfiguration
      discovery:
        bootstrapToken:
          token: '{{ .KubeadmTokens.BootstrapToken }}'
          unsafeSkipCAVerification: true
          apiServerEndpoint: "{{ .GetControlPlaneEndpoint "443" }}"
      nodeRegistration:
        taints: []
        kubeletExtraArgs:
          node-labels: ""
          feature-gates: ExperimentalCriticalPodAnnotation=true
  trustd:
    token: '{{ .TrustdInfo.Token }}'
    endpoints: [ {{ .Endpoints }} ]
`
