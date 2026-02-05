// 底层 SSE 连接调试测试
const TAVILY_API_KEY = "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";
const MCP_URL = `https://mcp.tavily.com/mcp/?tavilyApiKey=${TAVILY_API_KEY}`;

async function debugSSE() {
  console.log("=== SSE 底层调试 ===\n");
  console.log(`URL: ${MCP_URL}\n`);

  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 10000);

  try {
    console.log("1. 发送 GET 请求建立 SSE 连接...");
    const response = await fetch(MCP_URL, {
      method: "GET",
      headers: { "Accept": "text/event-stream" },
      signal: controller.signal
    });

    console.log(`   状态码: ${response.status}`);
    console.log(`   Content-Type: ${response.headers.get("content-type")}`);
    console.log(`   Headers:`, Object.fromEntries(response.headers.entries()));

    if (!response.body) {
      console.log("   ✗ 无响应体");
      return;
    }

    console.log("\n2. 读取 SSE 流...");
    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let chunks = 0;

    while (chunks < 5) {
      const { done, value } = await reader.read();
      if (done) {
        console.log("   流结束");
        break;
      }
      chunks++;
      const text = decoder.decode(value);
      console.log(`   [chunk ${chunks}]: ${text.slice(0, 200)}`);
    }

  } catch (error: any) {
    if (error.name === "AbortError") {
      console.log("\n✗ 请求超时 (10s)");
    } else {
      console.log("\n✗ 错误:", error.message);
    }
  } finally {
    clearTimeout(timeout);
  }
}

debugSSE();
