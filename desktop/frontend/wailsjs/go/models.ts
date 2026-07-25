export namespace engine {
	
	export class Anomaly {
	    type: string;
	    ref: string;
	    label?: string;
	    severity: string;
	    message: string;
	    oldFormula?: string;
	    newValue?: any;
	
	    static createFrom(source: any = {}) {
	        return new Anomaly(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.ref = source["ref"];
	        this.label = source["label"];
	        this.severity = source["severity"];
	        this.message = source["message"];
	        this.oldFormula = source["oldFormula"];
	        this.newValue = source["newValue"];
	    }
	}
	export class Mover {
	    ref: string;
	    label?: string;
	    oldValue: any;
	    newValue: any;
	    magnitude?: number;
	
	    static createFrom(source: any = {}) {
	        return new Mover(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ref = source["ref"];
	        this.label = source["label"];
	        this.oldValue = source["oldValue"];
	        this.newValue = source["newValue"];
	        this.magnitude = source["magnitude"];
	    }
	}
	export class Cascade {
	    origin: string;
	    originLabel?: string;
	    oldValue: any;
	    newValue: any;
	    affectedCount: number;
	    affected: string[];
	    topMovers: Mover[];
	
	    static createFrom(source: any = {}) {
	        return new Cascade(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.origin = source["origin"];
	        this.originLabel = source["originLabel"];
	        this.oldValue = source["oldValue"];
	        this.newValue = source["newValue"];
	        this.affectedCount = source["affectedCount"];
	        this.affected = source["affected"];
	        this.topMovers = this.convertValues(source["topMovers"], Mover);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CellChange {
	    coord: string;
	    row: number;
	    col: number;
	    label?: string;
	    classification: string;
	    oldValue: any;
	    newValue: any;
	    oldFormula?: string;
	    newFormula?: string;
	    displayFormat: string;
	    dependsOn: string[];
	    dependents: string[];
	    causedBy: string[];
	    magnitude?: number;
	
	    static createFrom(source: any = {}) {
	        return new CellChange(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.coord = source["coord"];
	        this.row = source["row"];
	        this.col = source["col"];
	        this.label = source["label"];
	        this.classification = source["classification"];
	        this.oldValue = source["oldValue"];
	        this.newValue = source["newValue"];
	        this.oldFormula = source["oldFormula"];
	        this.newFormula = source["newFormula"];
	        this.displayFormat = source["displayFormat"];
	        this.dependsOn = source["dependsOn"];
	        this.dependents = source["dependents"];
	        this.causedBy = source["causedBy"];
	        this.magnitude = source["magnitude"];
	    }
	}
	export class SheetDiff {
	    name: string;
	    changes: CellChange[];
	    rowsInserted: number[];
	    rowsDeleted: number[];
	
	    static createFrom(source: any = {}) {
	        return new SheetDiff(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.changes = this.convertValues(source["changes"], CellChange);
	        this.rowsInserted = source["rowsInserted"];
	        this.rowsDeleted = source["rowsDeleted"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Summary {
	    authoredCount: number;
	    computedCount: number;
	    sheetsAffected: string[];
	    narrative?: string;
	
	    static createFrom(source: any = {}) {
	        return new Summary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.authoredCount = source["authoredCount"];
	        this.computedCount = source["computedCount"];
	        this.sheetsAffected = source["sheetsAffected"];
	        this.narrative = source["narrative"];
	    }
	}
	export class VersionMeta {
	    label: string;
	    path: string;
	    committedAt: string;
	    author: string;
	
	    static createFrom(source: any = {}) {
	        return new VersionMeta(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.path = source["path"];
	        this.committedAt = source["committedAt"];
	        this.author = source["author"];
	    }
	}
	export class DiffResult {
	    schemaVersion: number;
	    from: VersionMeta;
	    to: VersionMeta;
	    summary: Summary;
	    sheets: SheetDiff[];
	    cascades: Cascade[];
	    anomalies: Anomaly[];
	    allSheets: string[];
	
	    static createFrom(source: any = {}) {
	        return new DiffResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schemaVersion = source["schemaVersion"];
	        this.from = this.convertValues(source["from"], VersionMeta);
	        this.to = this.convertValues(source["to"], VersionMeta);
	        this.summary = this.convertValues(source["summary"], Summary);
	        this.sheets = this.convertValues(source["sheets"], SheetDiff);
	        this.cascades = this.convertValues(source["cascades"], Cascade);
	        this.anomalies = this.convertValues(source["anomalies"], Anomaly);
	        this.allSheets = source["allSheets"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	

}

