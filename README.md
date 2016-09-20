# go-passthrough

A simple RabbitMQ message forwarder.

## What

This is a simple service that connects to a RabbitMQ broker, subscribes to the specified queues, and starts a process sending data to a graphite host. Multiple queues can be defined.

## Example


```RABBIT_URI="amqp://<user>:<password>@<rabbithost>:5672/" RABBIT_TAG="rabbit-carbon-passthrough@<myhost>" RABBIT_QUEUES="frelun" ./rabbit-carbon-passthrough ```

### Settings
* RABBIT_URI = The URI to the rabbit.
* RABBIT_TAG = Name of the connected system displayed in rabbit.
* RABBIT_QUEUES = Queue(s) to consume. Separate multiple queues with colon (:).

#### Optional settings
* RABBIT_ACK = Ack messages. This will removed messages from the rabbitmq. Default false
* EXIT_ERRORS = Exit after this amount of graphite send errors.
* STATUS_LISTEN = Listen ip/port for the status JSON API. Default "http://0.0.0.0:8082"
* GRAPHITE_HOST = Default host to send graphite data to. Default localhost.
* GRAPHITE_PORT = Default graphite port. Default 2003
* GRAPHITE_WRITE = Actually send messages to the graphite host. Default false.
*


