"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

const links = [
  { href: "/", label: "Endpoints" },
  { href: "/facilitators", label: "Facilitators" },
  { href: "/transactions", label: "Transactions" },
  { href: "/network", label: "Network" },
];

export function Nav() {
  const pathname = usePathname();

  return (
    <nav className="border-b border-border bg-void">
      <div className="max-w-7xl mx-auto px-4 h-12 flex items-center gap-8">
        <Link href="/" className="text-signal font-semibold tracking-tight">
          Agora
        </Link>
        <div className="flex gap-6">
          {links.map((link) => {
            const active = link.href === "/"
              ? pathname === "/"
              : pathname.startsWith(link.href);
            return (
              <Link
                key={link.href}
                href={link.href}
                className={`text-sm relative py-4 transition-colors after:absolute after:bottom-0 after:left-0 after:right-0 after:h-0.5 after:transition-colors ${
                  active
                    ? "text-ink-primary after:bg-signal"
                    : "text-ink-secondary hover:text-ink-primary after:bg-transparent hover:after:bg-ink-muted"
                }`}
              >
                {link.label}
              </Link>
            );
          })}
        </div>
      </div>
    </nav>
  );
}
