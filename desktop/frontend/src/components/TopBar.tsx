interface Props {
  workbook: string;
  scenario: string;
}

// GitHub-Desktop-style top bar: workbook (repo) ▾, scenario (branch) ▾, and a
// primary action. The dropdowns and button are decorative for the demo.
export function TopBar({ workbook, scenario }: Props) {
  return (
    <div className="topbar">
      <div className="tb picker" style={{ minWidth: 240 }}>
        <i className="ico">▤</i>
        <div style={{ flex: 1 }}>
          <div className="l">Current workbook</div>
          <div className="v">{workbook}</div>
        </div>
        <span className="caret">▾</span>
      </div>
      <div className="tb picker" style={{ minWidth: 210 }}>
        <i className="ico">⑂</i>
        <div style={{ flex: 1 }}>
          <div className="l">Current scenario</div>
          <div className="v">{scenario}</div>
        </div>
        <span className="caret">▾</span>
      </div>
      <div className="spacer" />
      <div className="tb action">
        <div className="synced">
          Synced 2m ago
          <br />
          via SharePoint
        </div>
        <button className="primary">Commit version</button>
      </div>
    </div>
  );
}
