"use client";

import {useEffect, useRef, useState} from "react";
import {usePathname} from "next/navigation";

/**
 * A thin progress bar at the top of the viewport that animates during
 * page navigations (Next.js App Router). Hooks into pathname changes.
 */
export default function NavigationProgress() {
  const pathname = usePathname();
  const [state, setState] = useState<"idle" | "loading" | "finishing">("idle");
  const prevPathname = useRef(pathname);
  const timeoutRef = useRef<ReturnType<typeof setTimeout>>(undefined);

  // Detect clicks on internal links to start the progress bar immediately
  useEffect(() => {
    function handleClick(e: MouseEvent) {
      const anchor = (e.target as HTMLElement).closest("a");
      if (!anchor) return;

      const href = anchor.getAttribute("href");
      if (!href || href.startsWith("http") || href.startsWith("#") || anchor.target === "_blank") return;

      // Internal navigation — start loading
      if (href !== pathname) {
        setState("loading");
      }
    }

    document.addEventListener("click", handleClick, {capture: true});
    return () => document.removeEventListener("click", handleClick, {capture: true});
  }, [pathname]);

  // When pathname changes, the navigation completed
  useEffect(() => {
    if (pathname !== prevPathname.current) {
      prevPathname.current = pathname;
      setState("finishing");

      clearTimeout(timeoutRef.current);
      timeoutRef.current = setTimeout(() => {
        setState("idle");
      }, 300);
    }

    return () => clearTimeout(timeoutRef.current);
  }, [pathname]);

  if (state === "idle") return null;

  return (
    <div className="fixed inset-x-0 top-0 z-[9999] h-0.5">
      <div
        className={`h-full bg-purple-500 ${
          state === "loading"
            ? "animate-progress-loading"
            : "animate-progress-finish"
        }`}
      />
    </div>
  );
}
