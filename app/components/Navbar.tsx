"use client";

import Image from "next/image";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useState } from "react";

const tabs = [
  { label: "Acasă", href: "/" },
  { label: "Linii", href: "/linii" },
  { label: "Stații", href: "/statii" },
  // { label: "Planifică", href: "/planifica" },
];

function getActiveIndex(pathname: string) {
  const idx = tabs.findIndex(
    (t) => t.href === "/" ? pathname === "/" : pathname.startsWith(t.href)
  );
  return idx >= 0 ? idx : 0;
}

export default function Navbar() {
  const pathname = usePathname();
  const [menuOpen, setMenuOpen] = useState(false);
  const activeIndex = getActiveIndex(pathname);

  return (
    <nav className="sticky top-0 z-50 border-b border-zinc-200 bg-white/80 backdrop-blur-md dark:border-zinc-800 dark:bg-zinc-950/80">
      <div className="mx-auto flex max-w-5xl items-center justify-between px-4 py-3">
        {/* Logo */}
        <Link href="/" className="flex items-center gap-2">
          <span className="inline-block animate-bus-bob">
            <Image src="/bus.svg" alt="Conexiuni Cluj" width={40} height={22} />
          </span>
          <span className="text-lg font-bold text-zinc-900 dark:text-white">
            Conexiuni Cluj
          </span>
        </Link>

        {/* Desktop tabs */}
        <div className="hidden md:flex items-center gap-1">
          {tabs.map((tab, i) => (
            <Link
              key={tab.href}
              href={tab.href}
              className={`px-4 py-2 rounded-md text-sm font-medium transition-colors ${
                activeIndex === i
                  ? "text-purple-600 bg-purple-50 dark:text-purple-400 dark:bg-purple-950/30"
                  : "text-zinc-500 hover:text-zinc-900 hover:bg-zinc-100 dark:text-zinc-400 dark:hover:text-white dark:hover:bg-zinc-800"
              }`}
            >
              {tab.label}
            </Link>
          ))}
        </div>

        {/* Mobile burger */}
        <button
          onClick={() => setMenuOpen(!menuOpen)}
          className="flex flex-col gap-1.5 p-2 md:hidden"
          aria-label="Toggle menu"
        >
          <span
            className={`block h-0.5 w-5 bg-zinc-700 transition-transform dark:bg-zinc-300 ${
              menuOpen ? "translate-y-2 rotate-45" : ""
            }`}
          />
          <span
            className={`block h-0.5 w-5 bg-zinc-700 transition-opacity dark:bg-zinc-300 ${
              menuOpen ? "opacity-0" : ""
            }`}
          />
          <span
            className={`block h-0.5 w-5 bg-zinc-700 transition-transform dark:bg-zinc-300 ${
              menuOpen ? "-translate-y-2 -rotate-45" : ""
            }`}
          />
        </button>
      </div>

      {/* Mobile menu */}
      <div
        className={`overflow-hidden transition-all duration-300 md:hidden ${
          menuOpen ? "max-h-60" : "max-h-0"
        }`}
      >
        <div className="flex flex-col border-t border-zinc-200 dark:border-zinc-800">
          {tabs.map((tab, i) => (
            <Link
              key={tab.href}
              href={tab.href}
              onClick={() => setMenuOpen(false)}
              className={`px-6 py-3 text-sm font-medium transition-colors ${
                activeIndex === i
                  ? "bg-purple-50 text-purple-600 dark:bg-purple-950/30 dark:text-purple-400"
                  : "text-zinc-600 hover:bg-zinc-50 dark:text-zinc-400 dark:hover:bg-zinc-900"
              }`}
            >
              {tab.label}
            </Link>
          ))}
        </div>
      </div>
    </nav>
  );
}
