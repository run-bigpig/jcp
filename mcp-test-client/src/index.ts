import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StreamableHTTPClientTransport } from "@modelcontextprotocol/sdk/client/streamableHttp.js";

const TAVILY_API_KEY = "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";
const MCP_URL = `https://mcp.tavily.com/mcp/?tavilyApiKey=${TAVILY_API_KEY}`;
const TIMEOUT_MS = 15000;

function withTimeout<T>(promise: Promise<T>, ms: number, name: string): Promise<T> {
  return Promise.race([
    promise,
    new Promise<T>((_, reject) =>
      setTimeout(() => reject(new Error(`${name} 超时 (${ms}ms)`)), ms)
    )
  ]);
}

async function main() {
  console.log("=== MCP Streamable HTTP 客户端测试 ===\n");
  console.log(`URL: ${MCP_URL}`);
  console.log(`超时: ${TIMEOUT_MS}ms\n`);

  const client = new Client(
    { name: "mcp-test-client", version: "1.0.0" },
    { capabilities: {} }
  );

  try {
    console.log("1. 创建 StreamableHTTP 传输层...");
    const transport = new StreamableHTTPClientTransport(new URL(MCP_URL));

    console.log("2. 连接到 MCP 服务器...");
    const start = Date.now();
    await withTimeout(client.connect(transport), TIMEOUT_MS, "连接");
    console.log(`   ✓ 连接成功! (${Date.now() - start}ms)\n`);

    console.log("3. 服务器能力:", client.getServerCapabilities(), "\n");

    console.log("4. 列出工具...");
    const { tools } = await withTimeout(client.listTools(), TIMEOUT_MS, "listTools");
    console.log(`   找到 ${tools.length} 个工具:`);
    tools.forEach(t => console.log(`   - ${t.name}`));

    console.log("\n5. 调用 search 工具...");
    const result = await withTimeout(
      client.callTool({ name: "search", arguments: { query: "test" } }),
      TIMEOUT_MS, "callTool"
    );
    console.log("   ✓ 成功!", result.content[0]?.type);

    console.log("\n=== 测试完成 ===");
    await client.close();
  } catch (error) {
    console.error("\n✗ 错误:", error);
    process.exit(1);
  }
}

main();
