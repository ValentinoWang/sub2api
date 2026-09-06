import { readConfig } from "./config";
import { launchRuntime, MembershipRunner } from "./runner";
import { createMembershipServer } from "./server";

async function main(): Promise<void> {
  const config = readConfig();
  const runtime = await launchRuntime();
  const server = createMembershipServer(config, new MembershipRunner(config, runtime));
  server.listen(config.listenPort, config.listenHost);
  const shutdown = async () => {
    server.close();
    await runtime.close();
  };
  process.once("SIGINT", shutdown);
  process.once("SIGTERM", shutdown);
}

void main();
