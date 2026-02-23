"use client";

import { useState } from "react";

interface SearchBarProps {
  placeholder?: string;
  onSearch?: (query: string) => void;
}

export function SearchBar({ placeholder = "Search endpoints by description, domain, or schema...", onSearch }: SearchBarProps) {
  const [query, setQuery] = useState("");

  return (
    <div className="w-full">
      <input
        type="text"
        value={query}
        onChange={(e) => {
          setQuery(e.target.value);
          onSearch?.(e.target.value);
        }}
        placeholder={placeholder}
        className="w-full bg-void-elevated border border-border rounded-sm px-4 py-3 text-sm text-ink-primary placeholder:text-ink-muted focus:outline-none focus:border-border-focus transition-colors font-mono"
      />
    </div>
  );
}
