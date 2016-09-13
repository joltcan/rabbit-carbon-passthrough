# go-passthrough

A simple RabbitMQ message forwarder.

## What

This is a simple service that connects to a RabbitMQ broker, subscribes to the specified queues, and starts a process with the message body passed to stdin.

## Example

The following example will consume messages from a queue and send it to a carbon writer instance. Multiple queues can be defined. See config.yml.example for details.

