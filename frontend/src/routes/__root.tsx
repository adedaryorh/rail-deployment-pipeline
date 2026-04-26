import { createRootRoute, Outlet } from "@tanstack/react-router";

export const Route = createRootRoute({
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