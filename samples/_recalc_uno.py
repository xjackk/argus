#!/usr/bin/env python3
"""Connect to a headless LibreOffice socket listener, open each workbook,
force a full recalculation, store (writing cached <v> values), and close.
Run with LibreOffice's bundled python so `uno` is importable."""
import sys, time, os
import uno
from com.sun.star.beans import PropertyValue

FILES = [
    "/Users/emo/projects/argus/samples/income_statement_v1.xlsx",
    "/Users/emo/projects/argus/samples/income_statement_v2.xlsx",
    "/Users/emo/projects/argus/samples/balance_sheet_v1.xlsx",
    "/Users/emo/projects/argus/samples/balance_sheet_v2.xlsx",
]

def connect(tries=40):
    localContext = uno.getComponentContext()
    resolver = localContext.ServiceManager.createInstanceWithContext(
        "com.sun.star.bridge.UnoUrlResolver", localContext)
    url = "uno:socket,host=localhost,port=2002;urp;StarOffice.ComponentContext"
    last = None
    for _ in range(tries):
        try:
            ctx = resolver.resolve(url)
            smgr = ctx.ServiceManager
            desktop = smgr.createInstanceWithContext("com.sun.star.frame.Desktop", ctx)
            return desktop
        except Exception as e:
            last = e
            time.sleep(0.5)
    raise SystemExit("could not connect to soffice: %s" % last)

def prop(name, value):
    p = PropertyValue()
    p.Name = name
    p.Value = value
    return p

def main():
    desktop = connect()
    for path in FILES:
        url = "file://" + path
        doc = desktop.loadComponentFromURL(url, "_blank", 0, (prop("Hidden", True),))
        doc.calculateAll()
        doc.store()
        doc.close(False)
        print("recalc+stored:", os.path.basename(path))
    print("ALL DONE")

main()
