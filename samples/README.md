# Argus sample workbooks — accounting & reporting

Realistic, richly-formatted 3-statement-style spreadsheets that demonstrate
Argus's diff engine + AI summary. Each statement has a **v1** and a **v2** that
differ by exactly **one authored input**, which then cascades through **dozens**
of cross-cell (and cross-sheet) formulas — the "blast radius" demo: change one
assumption and watch the ripple, then trace "how this number came about" back up
the dependency chain.

The Income Statement and Balance Sheet workbooks are **large and formatted**:
many line items × many periods, every value a formula, currency rendered as
`$#,##0` and margins/rates as `0.0%`. The number formats matter — the engine
surfaces each cell's `displayFormat`, and the AI narrator pre-renders every
figure through it, so the summary reads `$118,701` / `16.0%`, never a raw float.

Each file's formula cells carry **cached values** (recalculated by LibreOffice),
because the engine reads Excel's cached `<v>` and never recomputes.

Reproduce any result with:

```
cd /Users/emo/projects/argus
go run ./cmd/argus-diff [--narrate] samples/<name>_v1.xlsx samples/<name>_v2.xlsx
```

---

## 1. Income Statement — `income_statement_v1.xlsx` / `_v2.xlsx`

**What it models.** A 3-statement-style P&L across **two sheets**. An
`Assumptions` sheet holds **11 authored input cells** (blue): base revenue,
revenue growth %, gross margin %, SG&A / R&D / marketing / G&A / D&A as % of
revenue, interest rate, debt balance, tax rate — formatted `$#,##0` and `0.0%`.
A `P&L` sheet spans **8 periods across columns (FY20–FY27)** and **18 line items
down**: Revenue → COGS → Gross Profit → SG&A / R&D / Marketing / G&A → Total
OpEx → EBITDA → D&A → EBIT → Interest → Pretax → Tax → Net Income, plus Gross /
EBITDA / Net margin % rows. **Every P&L value is a formula** referencing
Assumptions (cross-sheet) and the prior period (cross-cell). Currency is
`$#,##0`; margin rows are `0.0%`.

**What changed v1→v2.** `Assumptions!B4` **Revenue Growth %: 10.0% → 16.0%**
(one cell). FY20 is a fixed base column and does not move; FY21–FY27 recompute
end to end.

**Engine diff.** **1 authored, 105 computed**, sheets `[Assumptions, P&L]`. The
single cascade from `Assumptions!B4` reaches every dollar line item across all
seven forecast periods (Revenue, COGS, Gross Profit, each OpEx line, Total OpEx,
EBITDA, D&A, EBIT, Pretax, Tax, Net Income). Interest Expense (a fixed
debt×rate) and the margin % rows are ratios that hold constant, so they
correctly stay out of the diff.

**AI narrative.**
> A user changed the Revenue Growth % input from 10.0% to 16.0%. That flowed
> through to Revenue, which rose from $81,846 to $118,701, with Cost of Goods
> Sold moving from $31,102 to $45,106 and Gross Profit from $50,745 to $73,595.
> Further down, Total Operating Expenses went from $28,646 to $41,545, EBITDA
> from $22,098 to $32,049, Operating Income (EBIT) from $18,825 to $27,301, D&A
> from $3,274 to $4,748, Pretax Income from $17,625 to $26,101, Income Tax from
> $4,230 to $6,264, and Net Income from $13,395 to $19,837.

---

## 2. Balance Sheet — `balance_sheet_v1.xlsx` / `_v2.xlsx`

**What it models.** A rolling balance sheet across **7 periods (FY20 opening +
FY21–FY26)** with **24 line items** on a `BalanceSheet` sheet, driven by an
`Assumptions` sheet of **20 authored input cells** (opening balances + annual
drivers). Assets (Cash, AR, Inventory, Prepaid → Total Current; Gross PP&E,
Accum. Depreciation, Net PP&E, Goodwill, Intangibles → Total Non-Current →
**Total Assets**); Liabilities (AP, Accrued, Deferred Revenue, Short-term Debt →
Total Current; Long-term Debt → **Total Liabilities**); Equity (Common Stock,
Retained Earnings → **Total Equity** → **Total L&E**). It **balances by
construction** via the accounting identity — Cash rolls from a cash-flow build,
PP&E from capex/depreciation, Debt from draws, Retained Earnings from net income
— so a **Balance Check (A − L&E)** row is `$0` in every period of both versions.

**What changed v1→v2.** `Assumptions!B17` **Annual Capex: $4,000 → $9,000** (one
cell). Capex is funded by an equal long-term-debt draw, so it lifts PP&E on the
asset side and debt on the funding side; both totals rise and the sheet stays
balanced.

**Engine diff.** **1 authored, 60 computed**, sheets `[Assumptions,
BalanceSheet]`. The cascade from `Assumptions!B17` moves Gross PP&E, Accumulated
Depreciation, Net PP&E, Total Non-Current Assets, Total Assets, Cash (via
depreciation), Long-term Debt, Total Liabilities, Total Equity and Total L&E
across all forecast periods — while the Balance Check row stays `$0`, so it is
(correctly) **absent from the diff**: direct proof the sheet still ties.

**AI narrative.**
> A human raised Annual Capex ($000) from $4,000 to $9,000. That flowed through
> to Gross PP&E, which went from $64,000 to $94,000, and to Net PP&E across
> periods — $19,600 to $39,100, $22,000 to $39,500, $24,000 to $39,000, and
> $25,600 to $37,600 — lifting Total Non-Current Assets from $40,600 to
> $60,100. On the funding side, Long-term Debt moved from $46,000 to $76,000,
> $42,000 to $67,000, and $38,000 to $58,000, bringing Total Liabilities from
> $65,400 to $95,400.

---

## 3. Cash Flow — `cash_flow_v1.xlsx` / `_v2.xlsx`

**What it models.** An indirect-method FY2024 statement of cash flows on one
sheet. Operating (Net Income + D&A ± working-capital changes → Cash from
Operations), Investing (CapEx → Cash from Investing), Financing (debt
repayment + dividends → Cash from Financing), then Net Change in Cash and
Beginning → Ending Cash Balance.

**What changed v1→v2.** `B12` **Capital Expenditures: −6,000 → −9,500** — a
CapEx ramp (one cell).

**Engine diff.** 1 authored, 3 computed, sheet `[Cash Flow]`, **2 anomalies**
(large-magnitude flags: Net Change in Cash swings 2,100 → −1,400, a sign flip;
and Ending Cash falls sharply).

| Cell | Label | Class | Old → New |
|------|-------|-------|-----------|
| B12 | Capital Expenditures | authored | −6,000 → −9,500 |
| B13 | Cash from Investing | computed | −6,000 → −9,500 |
| B20 | Net Change in Cash | computed | 2,100 → −1,400 |
| B22 | Ending Cash Balance | computed | 8,600 → 5,100 |

**AI narrative.**
> A human changed Capital Expenditures from -6000.00 to -9500.00. That flowed
> through to Cash from Investing, which moved from -6000.00 to -9500.00, and
> turned Net Change in Cash from 2100.00 to -1400.00. Ending Cash Balance came
> down from 8600.00 to 5100.00.

---

## How these were built

The Income Statement and Balance Sheet pairs are generated + formatted by
`_build_income_statement.py` and `_build_balance_sheet.py`, then recalculated by
LibreOffice via `_recalc_uno.py`. To regenerate:

1. Write labels, constants, formulas **and number formats** with `openpyxl`
   (formulas carry no cached values yet):
   ```
   cd samples
   python3 _build_income_statement.py
   python3 _build_balance_sheet.py
   ```
2. Recalculate and store cached `<v>` values. **Note:** LibreOffice's
   `--convert-to` does **not** recalculate on load, so use a scripted full
   recalc instead — launch a headless socket listener and drive it with the
   bundled python + UNO:
   ```
   soffice --headless --invisible --norestore \
     --accept="socket,host=localhost,port=2002;urp;StarOffice.ComponentContext" &
   /Applications/LibreOffice.app/Contents/Resources/python _recalc_uno.py
   pkill -f soffice
   ```
   `_recalc_uno.py` opens each workbook, calls `calculateAll()`, and `store()`s
   it — writing the cached `<v>` the engine reads.
3. Verify cached values landed (should print formula/value pairs, not empties):
   ```
   unzip -p <file>.xlsx 'xl/worksheets/*.xml' \
     | grep -o '<f[^>]*>[^<]*</f><v>[0-9.eE+-]*</v>' | head
   ```

If a file's formula cells lack numeric `<v>`, the diff shows 0 computed changes
— rerun the recalc step. All workbooks here were verified to carry cached values
and number formats (`$#,##0`, `0.0%`).
