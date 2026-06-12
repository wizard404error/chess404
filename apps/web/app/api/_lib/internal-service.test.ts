// @vitest-environment node
import { describe, expect, it } from "vitest";
import { buildUpstreamHeaders } from "./internal-service";

function makeRequest(headers: Record<string, string>, url = "https://web-production-9a697.up.railway.app/api/gateway/bootstrap"): Request {
  return new Request(url, { method: "POST", headers });
}

describe("buildUpstreamHeaders", () => {
  it("injects X-Forwarded-Proto and X-Forwarded-Host from the request URL when not present", () => {
    const req = makeRequest({ host: "web-production-9a697.up.railway.app" });
    const out = buildUpstreamHeaders(req);
    expect(out.get("x-forwarded-proto")).toBe("https");
    expect(out.get("x-forwarded-host")).toBe("web-production-9a697.up.railway.app");
  });

  it("sets Origin from the public URL when the browser did not send one", () => {
    const req = makeRequest({ host: "web-production-9a697.up.railway.app" });
    const out = buildUpstreamHeaders(req);
    expect(out.get("origin")).toBe("https://web-production-9a697.up.railway.app");
  });

  it("preserves the browser's Origin if it was set (cross-origin POST)", () => {
    const req = makeRequest({
      host: "web-production-9a697.up.railway.app",
      origin: "https://other.example.com",
    });
    const out = buildUpstreamHeaders(req);
    expect(out.get("origin")).toBe("https://other.example.com");
  });

  it("preserves existing X-Forwarded-Proto/Host from upstream proxy chain", () => {
    const req = makeRequest({
      host: "gateway.railway.internal",
      "x-forwarded-proto": "https",
      "x-forwarded-host": "web-production-9a697.up.railway.app",
    });
    const out = buildUpstreamHeaders(req);
    expect(out.get("x-forwarded-proto")).toBe("https");
    expect(out.get("x-forwarded-host")).toBe("web-production-9a697.up.railway.app");
    expect(out.get("origin")).toBe("https://web-production-9a697.up.railway.app");
  });

  it("strips hop-by-hop and oversized headers (host, connection, content-length)", () => {
    const req = makeRequest({
      host: "web-production-9a697.up.railway.app",
      connection: "close",
      "content-length": "1234",
      "x-custom": "keep-me",
    });
    const out = buildUpstreamHeaders(req);
    expect(out.get("host")).toBeNull();
    expect(out.get("connection")).toBeNull();
    expect(out.get("content-length")).toBeNull();
    expect(out.get("x-custom")).toBe("keep-me");
  });

  it("uses http when the request URL is http", () => {
    const req = makeRequest({ host: "localhost:3000" }, "http://localhost:3000/api/gateway/bootstrap");
    const out = buildUpstreamHeaders(req);
    expect(out.get("x-forwarded-proto")).toBe("http");
    expect(out.get("origin")).toBe("http://localhost:3000");
  });
});
