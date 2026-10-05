// Durable Object-scheduled containers (cf `schedulingPolicy: "durable-object"`) must be started
// with an explicit image, but @cloudflare/containers 0.3.7 calls start() without one. This wraps
// the runtime container so start() always passes the "default" image from cloudflare.config.ts.
// Remove once the library supports the new runtime.

type StartOptions = Omit<ContainerStartupOptions, "image" | "containerSnapshot">;

export function withExplicitImage(runtime: Container, imageName = "default"): Container {
  return new Proxy(runtime, {
    get(target, prop) {
      if (prop === "start") {
        return (options?: StartOptions) =>
          target.start({ enableInternet: true, ...options, image: target.images[imageName] });
      }
      const value = Reflect.get(target, prop, target);
      return typeof value === "function" ? value.bind(target) : value;
    },
  });
}
