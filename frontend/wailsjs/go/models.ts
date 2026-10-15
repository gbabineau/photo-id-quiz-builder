export namespace main {
	
	export class TripPreview {
	    tripName: string;
	    outputDir: string;
	    quizzes: trip.DayQuiz[];
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new TripPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tripName = source["tripName"];
	        this.outputDir = source["outputDir"];
	        this.quizzes = this.convertValues(source["quizzes"], trip.DayQuiz);
	        this.warnings = source["warnings"];
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
	export class TripRequest {
	    tripPath: string;
	    outputDir: string;
	
	    static createFrom(source: any = {}) {
	        return new TripRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tripPath = source["tripPath"];
	        this.outputDir = source["outputDir"];
	    }
	}

}

export namespace quiz {
	
	export class Species {
	    comName: string;
	    frequency?: number;
	    notes?: string;
	    speciesCode?: string;
	    sciName?: string;
	    category?: string;
	    taxonOrder?: number;
	    bandingCodes?: string[];
	    comNameCodes?: string[];
	    sciNameCodes?: string[];
	    order?: string;
	    familyCode?: string;
	    familyComName?: string;
	    familySciName?: string;
	
	    static createFrom(source: any = {}) {
	        return new Species(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.comName = source["comName"];
	        this.frequency = source["frequency"];
	        this.notes = source["notes"];
	        this.speciesCode = source["speciesCode"];
	        this.sciName = source["sciName"];
	        this.category = source["category"];
	        this.taxonOrder = source["taxonOrder"];
	        this.bandingCodes = source["bandingCodes"];
	        this.comNameCodes = source["comNameCodes"];
	        this.sciNameCodes = source["sciNameCodes"];
	        this.order = source["order"];
	        this.familyCode = source["familyCode"];
	        this.familyComName = source["familyComName"];
	        this.familySciName = source["familySciName"];
	    }
	}
	export class Quiz {
	    start_month: number;
	    end_month: number;
	    location: string;
	    notes?: string;
	    species: Species[];
	
	    static createFrom(source: any = {}) {
	        return new Quiz(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.start_month = source["start_month"];
	        this.end_month = source["end_month"];
	        this.location = source["location"];
	        this.notes = source["notes"];
	        this.species = this.convertValues(source["species"], Species);
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

export namespace trip {
	
	export class DayQuiz {
	    day: string;
	    fileName: string;
	    quiz: quiz.Quiz;
	
	    static createFrom(source: any = {}) {
	        return new DayQuiz(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.day = source["day"];
	        this.fileName = source["fileName"];
	        this.quiz = this.convertValues(source["quiz"], quiz.Quiz);
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

