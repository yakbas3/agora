"use client";

import { useState } from "react";

interface FilterGroup {
  label: string;
  options: string[];
}

interface FilterChipsProps {
  groups: FilterGroup[];
  onFilterChange?: (filters: Record<string, string | null>) => void;
}

export function FilterChips({ groups, onFilterChange }: FilterChipsProps) {
  const [active, setActive] = useState<Record<string, string | null>>({});

  const toggle = (group: string, option: string) => {
    const next = { ...active, [group]: active[group] === option ? null : option };
    setActive(next);
    onFilterChange?.(next);
  };

  return (
    <div className="flex flex-wrap gap-4">
      {groups.map((group) => (
        <div key={group.label} className="flex items-center gap-1.5">
          <span className="text-xs text-ink-tertiary">{group.label}:</span>
          {group.options.map((option) => (
            <button
              key={option}
              onClick={() => toggle(group.label, option)}
              className={`text-xs px-2 py-1 rounded-sm border transition-colors ${
                active[group.label] === option
                  ? "border-signal/40 bg-signal-dim/30 text-signal"
                  : "border-border text-ink-secondary hover:text-ink-primary hover:border-border-focus"
              }`}
            >
              {option}
            </button>
          ))}
        </div>
      ))}
    </div>
  );
}
