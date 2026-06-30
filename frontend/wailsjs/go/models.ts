export namespace main {
	
	export class CellError {
	    rowIndex: number;
	    colName: string;
	    value: string;
	    invalidChar: string;
	    charPos: number;
	    mapped: boolean;
	    mappedTo: string;
	    severity: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new CellError(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rowIndex = source["rowIndex"];
	        this.colName = source["colName"];
	        this.value = source["value"];
	        this.invalidChar = source["invalidChar"];
	        this.charPos = source["charPos"];
	        this.mapped = source["mapped"];
	        this.mappedTo = source["mappedTo"];
	        this.severity = source["severity"];
	        this.message = source["message"];
	    }
	}
	export class Field {
	    name: string;
	    type: string;
	    mapping?: string;
	    value: string;
	    options?: string[];
	
	    static createFrom(source: any = {}) {
	        return new Field(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.mapping = source["mapping"];
	        this.value = source["value"];
	        this.options = source["options"];
	    }
	}
	export class Settings {
	    colorScheme: string;
	    encoding: string;
	    charMapping: Record<string, string>;
	    fieldValues: Record<string, string>;
	    servicePrices: Record<string, string>;
	    recentFiles: string[];
	    windowX: number;
	    windowY: number;
	    windowW: number;
	    windowH: number;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.colorScheme = source["colorScheme"];
	        this.encoding = source["encoding"];
	        this.charMapping = source["charMapping"];
	        this.fieldValues = source["fieldValues"];
	        this.servicePrices = source["servicePrices"];
	        this.recentFiles = source["recentFiles"];
	        this.windowX = source["windowX"];
	        this.windowY = source["windowY"];
	        this.windowW = source["windowW"];
	        this.windowH = source["windowH"];
	    }
	}
	export class TableDataResult {
	    columns: string[];
	    rows: string[][];
	    cellErrors: CellError[];
	
	    static createFrom(source: any = {}) {
	        return new TableDataResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.columns = source["columns"];
	        this.rows = source["rows"];
	        this.cellErrors = this.convertValues(source["cellErrors"], CellError);
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

