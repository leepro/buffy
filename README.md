![Buffy image](https://github.com/leepro/buffy/blob/main/assets/buffy.png?raw=true)

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

    ```
    upstreams:
      - id: service1
        endpoint: http://localhost:9091
        interval: 2000 # msec
        autogate:
          uri: http://localhost:9091/_ping
          matches:
            - id: match1
              type: json
              if: status="*ok*" && num=10
              then: OPEN
            - id: match2
              type: json
              if: status="*error*"
              then: CLOSE          

      - id: service2
        endpoint: http://localhost:9092
        interval: 2000 # msec
        autogate:
          uri: http://localhost:9092/api/status
          matches:
            - id: match1
              type: json
              if: status="*ok*" && num=10
              then: OPEN
    ```

  * Endpoints
    ```
    endpoints:
      - id: example1
        desc: buffy endpoint
        path: /api/endpoint1/index.html
        type: proxy
        upstream:
          - service1
        proxy_mode: store_and_forward
        timeout: 20
        max_queue: 3
        methods:
          - GET
        response:
          - name: hit_timeout
            return_code: 503
            content: >
              {
                "status": "timeout",
                "desc": "timeout"
              }
          - name: hit_max_queue
            return_code: 503      
            content: >
              { 
                "status": "not enough resource",
                "max": 40
              }
      - id: example2
        desc: resource pool
        path: /api/endpoint2
        type: respond
        methods:
          - GET
        response:
          - name: ok
            return_code: 200
            content: >
              { "status": "ok", "desc": "example2", "_served": "{{URL}}", "_endpoint": "{{ID}}" }
          - name: not_found
            return_code: 400
            content: >
              { "status": "not found", "desc": "example2" }
      - id: example3
        desc: ping
        path: /api/ping
        type: respond
        methods:
          - GET
        response:
          - name: ok
            return_code: 200
            content: >
              { "status": "pong", "desc": "example3: ping/pong" }
          - name: not_found
            return_code: 404      
            content: >
              { "status": "not found", "desc": "example3: ping/pong" }
      - id: example4
        desc: file content
        path: /api/file
        type: respond
        methods:
          - GET
        response:
          - name: ok
            return_code: 200
            content: file:///file.json
    ```

* Installations
  * Standalone
  * Docker
  * Kubernetes
    * Helm chart

* Plugins
  * Streaming
  * Data Serving

* CI/CD
  * dev branch -> PR -> Approve -> Release (update license file)

* Maintainer
  * Kevin Lee

