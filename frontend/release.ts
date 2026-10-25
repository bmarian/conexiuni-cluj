import { execFileSync } from "node:child_process";

const PUSHES = "https://api.github.com/repos/bmarian/conexiuni-cluj/activity?activity_type=push&ref=refs/heads/master&per_page=100";
const FIRST_M3_COMMIT = "6066a2950362a9236f996fa30e2340b0aa1b0864";

// 3.{pushes to master since the Material 3 redesign that led to the commit being built}.0; empty when git or GitHub can't tell.
export async function releaseVersion(): Promise<string> {
  try {
    const built = new Set(execFileSync("git", ["rev-list", "HEAD", `^${FIRST_M3_COMMIT}^`]).toString().split("\n"));
    let count = 0;
    for (let url: string | undefined = PUSHES; url; ) {
      const response = await fetch(url, { headers: { Accept: "application/vnd.github+json" } });
      if (!response.ok) return "";
      const pushes = (await response.json()) as { after: string }[];
      count += pushes.filter((push) => built.has(push.after)).length;
      url = response.headers.get("link")?.match(/<([^>]+)>;\s*rel="next"/)?.[1];
    }
    return count ? `3.${count}.0` : "";
  } catch {
    return "";
  }
}
