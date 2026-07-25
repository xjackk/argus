# Argus sample workbooks — accounting & reporting

Realistic, everyday accounting/reporting spreadsheets (not finance-modeling
DCFs) that demonstrate Argus's diff engine + AI summary. Each statement has a
**v1** and a **v2** that differ by exactly **one authored input**, which then
cascades through cross-cell (and cross-sheet) formulas — the everyday workflow
where an accountant changes one assumption and needs to know what moved
downstream.

Each file's formula cells carry **cached values** (recalculated by LibreOffice),
because the engine reads Excel's cached `<v>` and never recomputes.

Reproduce any result with:

```
cd /Users/emo/projects/argus
go run ./cmd/argus-diff [--narrate] samples/<name>_v1.xlsx samples/<name>_v2.xlsx
```

---

## 1. Income Statement — `income_statement_v1.xlsx` / `_v2.xlsx`

**What it models.** A 3-period income statement (2023A / 2024E / 2025E) built
across **two sheets**: an `Assumptions` sheet (base revenue, growth %, COGS %,
SG&A %, R&D %, interest, tax rate) drives a `P&L` sheet via **cross-sheet
formulas**. Revenue → COGS → Gross Profit → Operating Expenses → Operating
Income (EBIT) → Interest → Pretax Income → Tax → Net Income, plus gross /
operating / net margin rows.

**What changed v1→v2.** `Assumptions!B4` **Revenue Growth %: 12.0% → 18.0%**
(one cell). The 2023A column is a fixed base and does not move; the 2024E and
2025E columns recompute end to end.

**Engine diff.** 1 authored, 22 computed, sheets `[Assumptions, P&L]`. The
single cascade originates at `Assumptions!B4` (Revenue Growth %) and reaches 26
downstream cells (revenue, COGS, gross profit, SG&A, R&D, EBIT, pretax, tax,
net income and all three margin rows, across both forecast years).

**AI narrative.**
> A user edited Revenue Growth % from 12.0% to 18.0%. That flowed through to
> Revenue (52684.80 → 58480.80), Cost of Goods Sold (30557.18 → 33918.86), and
> Gross Profit (22127.62 → 24561.94), while operating costs rose across SG&A
> (8956.42 → 9941.74), Research & Development (4741.63 → 5263.27), and Total
> Operating Expenses (13698.05 → 15205.01). Operating Income (EBIT) moved from
> 8429.57 to 9356.93, Pretax Income from 7229.57 to 8156.93, Income Tax from
> 1735.10 to 1957.66, and Net Income from 5494.47 to 6199.27.

---

## 2. Balance Sheet — `balance_sheet_v1.xlsx` / `_v2.xlsx`

**What it models.** A two-period balance sheet (FY2023 / FY2024) on one sheet.
Assets: Cash, Accounts Receivable, Inventory, PP&E → Total Assets.
Liabilities & Equity: Accounts Payable, Long-Term Debt, Common Stock, Retained
Earnings → Total L&E. **Cash is the balancing plug** (`Total L&E − (AR + Inv +
PP&E)`), so **Total Assets = Total L&E by construction** and a "Balance Check
(A − L&E)" row proves it ties to 0.

**What changed v1→v2.** `C12` **Long-Term Debt: 14,000 → 11,500** — a 2,500
debt paydown (one cell). Cash funds it, so both totals fall and the statement
stays balanced.

**Engine diff.** 1 authored, 3 computed, sheet `[Balance Sheet]`.

| Cell | Label | Class | Old → New |
|------|-------|-------|-----------|
| C12 | Long-Term Debt | authored | 14,000 → 11,500 |
| C4  | Cash & Equivalents | computed | 5,100 → 2,600 |
| C8  | Total Assets | computed | 45,000 → 42,500 |
| C15 | Total Liabilities & Equity | computed | 45,000 → 42,500 |

The Balance Check row stays 0 in both versions, so it is (correctly) **absent
from the diff** — direct proof the sheet still ties after the change.

**AI narrative.**
> A human edited Long-Term Debt from 14000.00 down to 11500.00. That flowed
> through to Cash & Equivalents, which recalculated from 5100.00 to 2600.00, a
> 2500.00 decrease matching the debt change. Both Total Assets and Total
> Liabilities & Equity moved from 45000.00 to 42500.00, and the two totals
> remain equal to each other.

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

1. `openpyxl` writes the labels, constants and formulas (formulas only — no
   cached values).
2. LibreOffice recalculates and stores cached `<v>`:
   ```
   soffice --headless --calc \
     --convert-to xlsx:"Calc MS Excel 2007 XML" --outdir samples <file>.xlsx
   ```
3. Verify cached values landed:
   ```
   unzip -p <file>.xlsx 'xl/worksheets/*.xml' \
     | grep -o '<f[^>]*>[^<]*</f><v>[^<]*</v>' | head
   ```

If a file's formula cells lack `<v>`, the diff would show 0 computed changes —
rerun the recalc step. All six files here were verified to carry cached values.
