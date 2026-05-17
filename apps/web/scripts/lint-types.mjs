import { spawn } from "node:child_process";

const isWindows = process.platform === "win32";
const pnpmBin = isWindows ? "pnpm.cmd" : "pnpm";

function run(args) {
  return new Promise((resolve, reject) => {
    const command = isWindows
      ? spawn(process.env.ComSpec || "cmd.exe", ["/d", "/s", "/c", `${pnpmBin} ${args.join(" ")}`], {
          cwd: process.cwd(),
          stdio: "inherit",
          shell: false,
        })
      : spawn(pnpmBin, args, {
          cwd: process.cwd(),
          stdio: "inherit",
          shell: false,
        });
    command.on("error", reject);
    command.on("exit", (code) => {
      if (code === 0) {
        resolve();
        return;
      }
      reject(new Error(`${pnpmBin} ${args.join(" ")} exited with code ${code ?? 1}`));
    });
  });
}

try {
  await run(["exec", "next", "typegen"]);
  await run(["exec", "tsc", "-p", "tsconfig.json", "--noEmit"]);
} catch (error) {
  console.error(error instanceof Error ? error.message : String(error));
  process.exitCode = 1;
}
