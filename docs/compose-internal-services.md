# Internal service exposure

The local Docker Compose stack should publish only the web application (`3000`) and API gateway (`8080`) to the host. Backend microservices communicate over the Compose network on container port `8080`, and MySQL remains internal on `3306`.
