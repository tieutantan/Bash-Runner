export namespace main {
	
	export class customRun {
	    name: string;
	    params: string;
	
	    static createFrom(source: any = {}) {
	        return new customRun(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.params = source["params"];
	    }
	}
	export class scriptView {
	    path: string;
	    displayName: string;
	    dir: string;
	    exists: boolean;
	    runs: customRun[];
	
	    static createFrom(source: any = {}) {
	        return new scriptView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.displayName = source["displayName"];
	        this.dir = source["dir"];
	        this.exists = source["exists"];
	        this.runs = this.convertValues(source["runs"], customRun);
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

