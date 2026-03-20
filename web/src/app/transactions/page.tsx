"use client";

import { useEffect, useState } from "react";
import { fetchTransactions, fetchFacilitators } from "@/lib/api";
import { Transaction, FacilitatorStats } from "@/lib/types";

function truncateAddress(addr: string): string {
  if (!addr || addr.length < 10) return addr;
  return `${addr.slice(0, 6)}...${addr.slice(-4)}`;
}

function timeAgo(dateStr: string): string {
  const now = new Date();
  const date = new Date(dateStr);
  const seconds = Math.floor((now.getTime() - date.getTime()) / 1000);
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

export default function TransactionsPage() {
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [facilitatorNames, setFacilitatorNames] = useState<string[]>([]);
  const [selectedFacilitator, setSelectedFacilitator] = useState("");
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(true);
  const limit = 50;

  useEffect(() => {
    fetchFacilitators()
      .then((data) => {
        const names = [...new Set(data.map((f: FacilitatorStats) => f.name))].sort();
        setFacilitatorNames(names);
      })
      .catch(console.error);
  }, []);

  useEffect(() => {
    setLoading(true);
    fetchTransactions(limit, offset, selectedFacilitator || undefined)
      .then((data) => {
        setTransactions(data.transactions || []);
        setTotal(data.total);
      })
      .catch(console.error)
      .finally(() => setLoading(false));
  }, [offset, selectedFacilitator]);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold tracking-tight">Transactions</h1>
          <p className="text-sm text-ink-secondary mt-1">
            <span className="font-mono text-ink-primary">{total.toLocaleString()}</span> V1 payment transactions indexed
          </p>
        </div>
        <select
          value={selectedFacilitator}
          onChange={(e) => { setSelectedFacilitator(e.target.value); setOffset(0); }}
          className="bg-void-elevated border border-border rounded-md px-3 py-1.5 text-sm text-ink-primary"
        >
          <option value="">All facilitators</option>
          {facilitatorNames.map((name) => (
            <option key={name} value={name}>{name}</option>
          ))}
        </select>
      </div>

      {loading ? (
        <div className="text-sm text-ink-secondary">Loading transactions...</div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-xs text-ink-secondary border-b border-border">
                <th className="pb-2 pr-4">Tx Hash</th>
                <th className="pb-2 pr-4">Facilitator</th>
                <th className="pb-2 pr-4">From</th>
                <th className="pb-2 pr-4">To</th>
                <th className="pb-2 pr-4 text-right">Amount</th>
                <th className="pb-2 text-right">Time</th>
              </tr>
            </thead>
            <tbody>
              {transactions.map((tx) => (
                <tr key={tx.id} className="border-b border-border-soft hover:bg-void-elevated/50">
                  <td className="py-2 pr-4 font-mono text-xs">
                    <a
                      href={`https://basescan.org/tx/${tx.tx_hash}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-signal hover:underline"
                    >
                      {truncateAddress(tx.tx_hash)}
                    </a>
                  </td>
                  <td className="py-2 pr-4 text-xs">{tx.facilitator_name}</td>
                  <td className="py-2 pr-4 font-mono text-xs text-ink-secondary">{truncateAddress(tx.payer_address)}</td>
                  <td className="py-2 pr-4 font-mono text-xs text-ink-secondary">{truncateAddress(tx.recipient_address)}</td>
                  <td className="py-2 pr-4 text-right font-mono text-xs">
                    ${tx.amount_usd.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 6 })}
                  </td>
                  <td className="py-2 text-right text-xs text-ink-secondary">{timeAgo(tx.block_time)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Pagination */}
      <div className="flex items-center justify-between text-sm">
        <span className="text-ink-secondary">
          Showing {offset + 1}-{Math.min(offset + limit, total)} of {total.toLocaleString()}
        </span>
        <div className="flex gap-2">
          <button
            onClick={() => setOffset(Math.max(0, offset - limit))}
            disabled={offset === 0}
            className="px-3 py-1 border border-border rounded-md text-ink-secondary hover:text-ink-primary disabled:opacity-50"
          >
            Prev
          </button>
          <button
            onClick={() => setOffset(offset + limit)}
            disabled={offset + limit >= total}
            className="px-3 py-1 border border-border rounded-md text-ink-secondary hover:text-ink-primary disabled:opacity-50"
          >
            Next
          </button>
        </div>
      </div>
    </div>
  );
}
