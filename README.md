Buffy: a backend proxy for CI/CD as a buffer

* Features
  * REST API endpoints for testing or returning a predefined simple content (e.g. JSON)
  * Add multiple listeners
  * Add webhook on connections for a listener
  * Add proxies (upstreams)

* Building blocks
  * Listeners
  
  ```
  buffy:
    listen:
      port: 7000
      bind: 0.0.0.0
    admin:
      path: /_admin
      port: 7001
      bind: 0.0.0.0
      notify:
        webhook: http://localhost:6666
        slack:
  ```
 
  * Upstreams
  * Endpoints

* CI/CD
  * dev branch -> PR -> Approve -> Release (update license file)

* Maintainer
  * Kevin Lee
