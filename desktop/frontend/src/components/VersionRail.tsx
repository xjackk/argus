import { useState } from "react";
import type { CommitRow } from "../data/history";

interface Props {
  commits: CommitRow[];
  selectedId: string;
  onSelect: (id: string) => void;
}

export function VersionRail({ commits, selectedId, onSelect }: Props) {
  const [tab, setTab] = useState<"changes" | "history">("history");
  const [filter, setFilter] = useState("");

  const rows = commits.filter((c) =>
    (c.message + " " + c.author).toLowerCase().includes(filter.toLowerCase())
  );

  return (
    <div className="rail">
      <div className="rail-tabs">
        <div
          className={"rail-tab" + (tab === "changes" ? " on" : "")}
          onClick={() => setTab("changes")}
        >
          Changes
        </div>
        <div
          className={"rail-tab" + (tab === "history" ? " on" : "")}
          onClick={() => setTab("history")}
        >
          History
        </div>
      </div>
      <div className="rail-filter">
        <input
          placeholder="Filter versions…"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
        />
      </div>
      <div className="rail-list">
        {rows.map((c) => (
          <div
            key={c.id}
            className={"crow" + (c.id === selectedId ? " sel" : "")}
            onClick={() => onSelect(c.id)}
          >
            <div className="csum">{c.message}</div>
            <div className="cmeta">
              <div className="av" />
              <span className="cby">
                {c.author} · {c.when}
              </span>
            </div>
            <div className="cnt">
              {c.base ? (
                <span className="tag-m">base version</span>
              ) : c.structural ? (
                <span className="tag-m">structural — row inserted</span>
              ) : (
                <>
                  <span className="tag-a">{c.authoredCount} authored</span>
                  <span className="tag-m">·</span>
                  <span className="tag-c">{c.computedCount} computed</span>
                </>
              )}
              {c.signedOff && (
                <span className="signoff" title={`Signed off by ${c.signedOff}`}>
                  ✓ signed
                </span>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
