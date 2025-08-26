# headscale upgrade report: from 0.22.3 to 0.23.0

## headscale is used in vpncloud-service-controlplane

which is used in QuickLinkUltra

our control plane is based on headscale v0.22.3, currently headscale has upgraded to v0.26.1

and there are a large refactor when upgrading to v0.23.0

many of our features is easy to implement due to the flat project structure, but porting those features is not easy.

my key achievement is to extract all the moxa headscale features in v0.22.3 and port them to v0.23.0(rebased)

here is the UPstream report and refactor report
https://wiki.moxa.com/spaces/~JieYC_Chung/pages/434247339/Headscale+v0.23+Upgrade+Refactor+Report

# Todo List
7/w2
- [X] analized the project structure ,and what different between v0.22.3 and v0.23.0
- [X] extract all the moxa headscale features in v0.22.3
- [x] build headscale-cluster local deployment
7/w3
- [x] Sync by DB for Cluster
- [x] ACL Config Auto reload
- [x] ACL Related APIs
7/w4
- [x] Add routes/peers cache mechanism for reduce db query
- [x] Add PreAuth Key Tag Lock mechanism
8/w1
- [x] Confirm whether v0.23.0 supports “PeersRemoved” in MapResponse
  - [x] Adjust routes mechanism
    -

  8/w2
- [x] Binding Node with PreAuth Key
- [X] PreAuthKey Related APIs
- [X] Implement API for Disconnect Actively
  - [X] ExpireNode Related APIs
- [X] assign vpn ip by tag

- [X] Support for transparent mode
  - [X] include destination node's routes when parse ACL Policy
    - 8/w3
- [X] Support environment variable format in the db settings of config.yaml
  - 8/w3

## analized the project structure ,and what different between v0.22.3 and v0.23.0
headscale upgrade and refactor report
https://wiki.moxa.com/spaces/~JieYC_Chung/pages/434247339/Headscale+v0.23+Upgrade+Refactor+Report
