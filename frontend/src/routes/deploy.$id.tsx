import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useRef, useState } from "react";
import { Link, useNavigate } from "@tanstack/react-router";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";

export const Route = createFileRoute("/deploy/$id")({
    component: DeployPage,
});

interface Deploy {
    id: string;
    state: string;
    image_tag: string;
    created_at: string;
    error?: string;
}

const TERMINAL_DONE_STATES = ["running", "failed", "rolled_back"];

export default function DeployPage() {
    const { id } = Route.useParams();
    const navigate = useNavigate();
    const qc = useQueryClient();

    const [lines, setLines] = useState<string[]>([]);
    const [streamDone, setStreamDone] = useState(false);
    const bottomRef = useRef<HTMLDivElement>(null);

    const { data: deploy } = useQuery<Deploy>({
        queryKey: ["deploy", id],
        queryFn: async () => {
            const res = await fetch(`/api/deploys`);
            const all: Deploy[] = await res.json();
            const found = all.find((d) => d.id === id);
            if (!found) throw new Error("Deploy not found");
            return found;
        },
        refetchInterval: (q) =>
            q.state.data && TERMINAL_DONE_STATES.includes(q.state.data.state) ? false : 2_000,
    });

    useEffect(() => {
        const es = new EventSource(`/api/deploys/${id}/logs`);
        es.onmessage = (e) => {
            setLines((prev) => [...prev, e.data]);
        };
        es.addEventListener("done", () => {
            setStreamDone(true);
            es.close();
        });
        es.onerror = () => {
            setStreamDone(true);
            es.close();
        };
        return () => es.close();
    }, [id]);

    useEffect(() => {
        bottomRef.current?.scrollIntoView({ behavior: "smooth" });
    }, [lines]);

    const { mutate: rollback, isPending: rollingBack } = useMutation({
        mutationFn: async () => {
            const res = await fetch(`/api/deploys/${id}/rollback`, { method: "POST" });
            if (!res.ok) {
                const body = await res.json();
                throw new Error(body.error || "Rollback failed");
            }
            return res.json();
        },
        onSuccess: () => {
            qc.invalidateQueries({ queryKey: ["deploys"] });
            qc.invalidateQueries({ queryKey: ["deploy", id] });
        },
    });

    const stateColor: Record<string, string> = {
        pending: "var(--muted)",
        building: "var(--yellow)",
        running: "var(--green)",
        failed: "var(--red)",
        rolled_back: "var(--blue)",
    };

    return (
        <div>
            <div style={{ display: "flex", alignItems: "center", gap: "1rem", marginBottom: "1.5rem" }}>
                <Link to="/" style={{ fontSize: "0.85rem", color: "var(--muted)" }}>
                    ← Back
                </Link>
                <h2 style={{ fontSize: "1rem", fontWeight: 600, fontFamily: "monospace" }}>
                    deploy/{id.slice(0, 8)}
                </h2>
                {deploy && (
                    <span
                        style={{
                            color: stateColor[deploy.state] ?? "var(--muted)",
                            fontWeight: 700,
                            fontSize: "0.82rem",
                            textTransform: "uppercase",
                            letterSpacing: "0.05em",
                        }}
                    >
            {deploy.state}
          </span>
                )}
                {deploy?.state === "running" && (
                    <button
                        onClick={() => rollback()}
                        disabled={rollingBack}
                        style={{
                            marginLeft: "auto",
                            background: "var(--red)",
                            color: "#fff",
                            border: "none",
                            borderRadius: 6,
                            padding: "0.4rem 1rem",
                            fontSize: "0.85rem",
                            fontWeight: 600,
                            cursor: rollingBack ? "not-allowed" : "pointer",
                        }}
                    >
                        {rollingBack ? "Rolling back…" : "↩ Rollback"}
                    </button>
                )}
            </div>

            {deploy?.error && (
                <div
                    style={{
                        background: "rgba(239,68,68,0.1)",
                        border: "1px solid var(--red)",
                        borderRadius: 6,
                        padding: "0.75rem 1rem",
                        marginBottom: "1rem",
                        fontSize: "0.85rem",
                        color: "var(--red)",
                    }}
                >
                    {deploy.error}
                </div>
            )}

            <div
                style={{
                    background: "var(--surface)",
                    border: "1px solid var(--border)",
                    borderRadius: 8,
                    padding: "1rem",
                    fontFamily: "monospace",
                    fontSize: "0.82rem",
                    lineHeight: 1.6,
                    height: "65vh",
                    overflowY: "auto",
                    whiteSpace: "pre-wrap",
                    wordBreak: "break-all",
                }}
            >
                {lines.length === 0 && !streamDone && (
                    <span style={{ color: "var(--muted)" }}>Connecting to log stream…</span>
                )}
                {lines.map((line, i) => (
                    <div
                        key={i}
                        style={{
                            color: line.startsWith("==> ERROR") ? "var(--red)"
                                : line.startsWith("==>") ? "var(--green)"
                                    : "var(--text)",
                        }}
                    >
                        {line}
                    </div>
                ))}
                {streamDone && (
                    <div style={{ color: "var(--muted)", marginTop: "0.5rem" }}>
                        ── stream ended ──
                    </div>
                )}
                <div ref={bottomRef} />
            </div>
        </div>
    );
}