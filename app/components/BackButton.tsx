"use client";

import {useRouter} from "next/navigation";

export default function BackButton({fallbackHref, fallbackLabel}: {
  fallbackHref: string;
  fallbackLabel: string;
}) {
  const router = useRouter();

  return (
    <button
      onClick={() => {
        // If there's browser history, go back. Otherwise navigate to the fallback.
        if (window.history.length > 1) {
          router.back();
        } else {
          router.push(fallbackHref);
        }
      }}
      className="inline-flex items-center gap-1 text-sm text-zinc-500 transition-colors hover:text-zinc-900 dark:text-zinc-400 dark:hover:text-white"
    >
      {fallbackLabel}
    </button>
  );
}
