import { createRouter, createRootRoute, createRoute, Outlet } from "@tanstack/react-router";
import IndexPage from "./routes/index";
import DeployPage from "./routes/deploy.$id";

const rootRoute = createRootRoute({
  component: () => (
    <div style={{ maxWidth: 900, margin: "0 auto", padding: "2rem 1rem" }}>
      <header style={{ marginBottom: "2rem", borderBottom: "1px solid var(--border)", paddingBottom: "1rem" }}>
        <h1 style={{ fontSize: "1.4rem", fontWeight: 700, letterSpacing: "-0.02em" }}>
          🚀 Deployment Pipeline
        </h1>
      </header>
      <Outlet />
    </div>
  ),
});


const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: IndexPage,
});

const deployRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/deploy/$id",
  component: DeployPage,
});

const routeTree = rootRoute.addChildren([indexRoute, deployRoute]);

export const router = createRouter({ routeTree });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}

