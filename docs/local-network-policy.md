# Local networking policy

For Docker Compose, only browser-facing entry points should publish host ports. Backend services stay internal to the Compose network and are accessed by service DNS names.
