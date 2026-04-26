import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { createFileRoute, Link } from "@tanstack/react-router";




interface Deploy {
  id: string;
  state: "pending" | "building" | "running" | "failed" | "rolled_back";
  image_tag: string;
  created_at: string;
  updated_at: string;
  error?: string;
}

const STATE_COLORS: Record<Deploy["state"], string> = {
  pending: "var(--muted)",
  building: "var(--yellow)",
  running: "var(--green)",
  failed: "var(--red)",
  rolled_back: "var(--blue)",
};
interface Deploy {
    id: string;
    state: "pending" | "building" | "running" | "failed" | "rolled_back";
    image_tag: string;
    created_at: string;
    updated_at: string;
    error?: string;
}

export const Route = createFileRoute("/")({
    component: IndexPage,
});


async function fetchDeploys(): Promise<Deploy[]> {
  const res = await fetch("/api/deploys");
  if (!res.ok) throw new Error("Failed to fetch deploys");
  return res.json();
}

async function triggerDeploy(): Promise<Deploy> {
  const res = await fetch("/api/deploy", { method: "POST" });
  if (!res.ok) throw new Error("Failed to trigger deploy");
  return res.json();
}

export default function IndexPage() {
  const qc = useQueryClient();
  const [error, setError] = useState<string | null>(null);

  const { data: deploys = [], isLoading } = useQuery({
    queryKey: ["deploys"],
    queryFn: fetchDeploys,
    refetchInterval: 3_000,
  });

  const { mutate: deploy, isPending } = useMutation({
    mutationFn: triggerDeploy,
    onSuccess: () => {
      setError(null);
      qc.invalidateQueries({ queryKey: ["deploys"] });
    },
    onError: (e: Error) => setError(e.message),
  });

  return (
    <div>
      <div style={{ display: "flex", alignItems: "center", gap: "1rem", marginBottom: "2rem" }}>
        <button
          onClick={() => deploy()}
          disabled={isPending}
          style={{
            background: isPending ? "var(--border)" : "var(--accent)",
            color: "#fff",
            border: "none",
            borderRadius: 8,
            padding: "0.65rem 1.5rem",
            fontSize: "0.95rem",
            fontWeight: 600,
            cursor: isPending ? "not-allowed" : "pointer",
            transition: "opacity 0.15s",
          }}
        >
          {isPending ? "Deploying…" : "▶ Deploy"}
        </button>
        {error && <span style={{ color: "var(--red)", fontSize: "0.85rem" }}>{error}</span>}
      </div>

      <h2 style={{ fontSize: "1rem", fontWeight: 600, marginBottom: "1rem", color: "var(--muted)" }}>
        DEPLOYMENTS
      </h2>

      {isLoading ? (
        <p style={{ color: "var(--muted)" }}>Loading…</p>
      ) : deploys.length === 0 ? (
        <p style={{ color: "var(--muted)" }}>No deployments yet. Hit Deploy to start one.</p>
      ) : (
        <table style={{ width: "100%", borderCollapse: "collapse", fontSize: "0.9rem" }}>
          <thead>
            <tr style={{ borderBottom: "1px solid var(--border)", color: "var(--muted)", textAlign: "left" }}>
              <th style={{ padding: "0.5rem 0.75rem" }}>ID</th>
              <th style={{ padding: "0.5rem 0.75rem" }}>State</th>
              <th style={{ padding: "0.5rem 0.75rem" }}>Image</th>
              <th style={{ padding: "0.5rem 0.75rem" }}>Started</th>
              <th style={{ padding: "0.5rem 0.75rem" }}>Logs</th>
            </tr>
          </thead>
          <tbody>
            {[...deploys].reverse().map((d) => (
              <tr
                key={d.id}
                style={{ borderBottom: "1px solid var(--border)" }}
              >
                <td style={{ padding: "0.6rem 0.75rem", fontFamily: "monospace", fontSize: "0.8rem", color: "var(--muted)" }}>
                  {d.id.slice(0, 8)}
                </td>
                <td style={{ padding: "0.6rem 0.75rem" }}>
                  <span
                    style={{
                      color: STATE_COLORS[d.state],
                      fontWeight: 600,
                      fontSize: "0.82rem",
                      textTransform: "uppercase",
                      letterSpacing: "0.04em",
                    }}
                  >
                    {d.state}
                  </span>
                </td>
                <td style={{ padding: "0.6rem 0.75rem", fontFamily: "monospace", fontSize: "0.8rem" }}>
                  {d.image_tag || "—"}
                </td>
                <td style={{ padding: "0.6rem 0.75rem", color: "var(--muted)", fontSize: "0.82rem" }}>
                  {new Date(d.created_at).toLocaleTimeString()}
                </td>
                <td style={{ padding: "0.6rem 0.75rem" }}>
                  <Link to="/deploy/$id" params={{ id: d.id }} style={{ fontSize: "0.82rem" }}>
                    View logs →
                  </Link>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
