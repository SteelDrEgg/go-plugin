#!/usr/bin/env python3
from concurrent import futures
import os
import sys
import time

import grpc
from grpc_health.v1 import health_pb2, health_pb2_grpc
from grpc_health.v1.health import HealthServicer

PROTO_DIR = os.path.join(os.path.dirname(__file__), "proto")
if PROTO_DIR not in sys.path:
    sys.path.insert(0, PROTO_DIR)

import greeter_pb2
import greeter_pb2_grpc

MAGIC_COOKIE_KEY = "GREETER_GRPC_PLUGIN"
MAGIC_COOKIE_VALUE = "hello"


class GreeterServicer(greeter_pb2_grpc.GreeterServicer):
    def SayHello(self, request, context):
        return greeter_pb2.GreetReply(message=f"hello from python grpc plugin: {request.name}")


class GreeterCallbackServicer(greeter_pb2_grpc.GreeterCallbackServicer):
    def SayHello(self, request, context):
        return greeter_pb2.GreetReply(message=f"hello from python grpc plugin: {request.name}")

    def Chat(self, request_iterator, context):
        yield greeter_pb2.ChatMessage(**{"from": "plugin", "text": "stream callback ready (python)"})
        for msg in request_iterator:
            yield greeter_pb2.ChatMessage(**{"from": "plugin", "text": f"echo: {msg.text}"})

    def NotifyHost(self, request, context):
        context.abort(grpc.StatusCode.UNIMPLEMENTED, "python plugin does not support broker callback")

    def NotifyHostStd(self, request, context):
        with grpc.insecure_channel(request.host_addr) as channel:
            client = greeter_pb2_grpc.HostCallbackStub(channel)
            reply = client.OnEvent(
                greeter_pb2.CallbackEventRequest(
                    text=f"callback from python plugin via standard grpc: {request.message}",
                    token=request.token,
                )
            )
        return greeter_pb2.NotifyHostStdReply(result=f"host callback ack: {reply.ack}")


def validate_magic_cookie():
    actual = os.getenv(MAGIC_COOKIE_KEY)
    if actual == MAGIC_COOKIE_VALUE:
        return
    print(
        f"invalid magic cookie: env[{MAGIC_COOKIE_KEY}]={actual!r}",
        file=sys.stderr,
        flush=True,
    )
    sys.exit(1)


def serve():
    validate_magic_cookie()

    health = HealthServicer()
    health.set("plugin", health_pb2.HealthCheckResponse.SERVING)

    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    greeter_pb2_grpc.add_GreeterServicer_to_server(GreeterServicer(), server)
    greeter_pb2_grpc.add_GreeterCallbackServicer_to_server(GreeterCallbackServicer(), server)
    health_pb2_grpc.add_HealthServicer_to_server(health, server)

    port = server.add_insecure_port("127.0.0.1:0")
    if port <= 0:
        raise RuntimeError("failed to bind plugin gRPC server")

    server.start()

    # Hashicorp go-plugin handshake:
    # CORE|APP|NETWORK|ADDR|PROTOCOL
    print(f"1|1|tcp|127.0.0.1:{port}|grpc")
    sys.stdout.flush()

    try:
        while True:
            time.sleep(24 * 60 * 60)
    except KeyboardInterrupt:
        server.stop(0)


if __name__ == "__main__":
    serve()
