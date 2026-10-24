export namespace backup {
	
	export class DriveFile {
	    fileId?: number;
	    fileName: string;
	    path: string;
	    sizeBytes: number;
	    mimeType: string;
	    modifiedAt: string;
	    fileStatus: string;
	    aiStatus: string;
	    description?: string;
	
	    static createFrom(source: any = {}) {
	        return new DriveFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fileId = source["fileId"];
	        this.fileName = source["fileName"];
	        this.path = source["path"];
	        this.sizeBytes = source["sizeBytes"];
	        this.mimeType = source["mimeType"];
	        this.modifiedAt = source["modifiedAt"];
	        this.fileStatus = source["fileStatus"];
	        this.aiStatus = source["aiStatus"];
	        this.description = source["description"];
	    }
	}

}

export namespace service {
	
	export class AIBatchTimings {
	    preparationMs: number;
	    markStatusMs: number;
	    workerMs: number;
	    saveResultsMs: number;
	    totalMs: number;
	    workerStartupMs: number;
	    workerReused: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AIBatchTimings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.preparationMs = source["preparationMs"];
	        this.markStatusMs = source["markStatusMs"];
	        this.workerMs = source["workerMs"];
	        this.saveResultsMs = source["saveResultsMs"];
	        this.totalMs = source["totalMs"];
	        this.workerStartupMs = source["workerStartupMs"];
	        this.workerReused = source["workerReused"];
	    }
	}
	export class AIFailure {
	    fileId: number;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new AIFailure(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fileId = source["fileId"];
	        this.message = source["message"];
	    }
	}
	export class AIResult {
	    fileId: number;
	    description: string;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new AIResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fileId = source["fileId"];
	        this.description = source["description"];
	        this.status = source["status"];
	    }
	}
	export class AIBatchResult {
	    results: AIResult[];
	    failures: AIFailure[];
	    timings: AIBatchTimings;
	
	    static createFrom(source: any = {}) {
	        return new AIBatchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.results = this.convertValues(source["results"], AIResult);
	        this.failures = this.convertValues(source["failures"], AIFailure);
	        this.timings = this.convertValues(source["timings"], AIBatchTimings);
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
	
	
	
	export class AISetupStatus {
	    status: string;
	    phase: string;
	    detail: string;
	    completed: number;
	    total: number;
	    unit: string;
	    device: string;
	    error: string;
	    skipped: string[];
	
	    static createFrom(source: any = {}) {
	        return new AISetupStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.phase = source["phase"];
	        this.detail = source["detail"];
	        this.completed = source["completed"];
	        this.total = source["total"];
	        this.unit = source["unit"];
	        this.device = source["device"];
	        this.error = source["error"];
	        this.skipped = source["skipped"];
	    }
	}
	export class AppError {
	    code: string;
	    message: string;
	    details?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new AppError(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.message = source["message"];
	        this.details = source["details"];
	    }
	}
	export class BackupItemResult {
	    sourcePath: string;
	    destinationPath?: string;
	    status: string;
	    error?: AppError;
	
	    static createFrom(source: any = {}) {
	        return new BackupItemResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sourcePath = source["sourcePath"];
	        this.destinationPath = source["destinationPath"];
	        this.status = source["status"];
	        this.error = this.convertValues(source["error"], AppError);
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
	export class BackupJob {
	    id: number;
	    sourcePath: string;
	    destinationPath: string;
	    startedAt: string;
	    completedAt?: string;
	    totalFiles: number;
	    successCount: number;
	    failedCount: number;
	    skippedCount: number;
	    status: string;
	    durationMs: number;
	
	    static createFrom(source: any = {}) {
	        return new BackupJob(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sourcePath = source["sourcePath"];
	        this.destinationPath = source["destinationPath"];
	        this.startedAt = source["startedAt"];
	        this.completedAt = source["completedAt"];
	        this.totalFiles = source["totalFiles"];
	        this.successCount = source["successCount"];
	        this.failedCount = source["failedCount"];
	        this.skippedCount = source["skippedCount"];
	        this.status = source["status"];
	        this.durationMs = source["durationMs"];
	    }
	}
	export class BackupProgress {
	    jobId: number;
	    completed: number;
	    total: number;
	    currentPath?: string;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new BackupProgress(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.jobId = source["jobId"];
	        this.completed = source["completed"];
	        this.total = source["total"];
	        this.currentPath = source["currentPath"];
	        this.status = source["status"];
	    }
	}
	export class BackupRequest {
	    mode?: string;
	    sourcePath: string;
	    destinationPath: string;
	    filePaths: string[];
	
	    static createFrom(source: any = {}) {
	        return new BackupRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.sourcePath = source["sourcePath"];
	        this.destinationPath = source["destinationPath"];
	        this.filePaths = source["filePaths"];
	    }
	}
	export class BackupResult {
	    jobId: number;
	    totalFiles: number;
	    successCount: number;
	    skippedCount: number;
	    failedCount: number;
	    durationMs: number;
	    items: BackupItemResult[];
	
	    static createFrom(source: any = {}) {
	        return new BackupResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.jobId = source["jobId"];
	        this.totalFiles = source["totalFiles"];
	        this.successCount = source["successCount"];
	        this.skippedCount = source["skippedCount"];
	        this.failedCount = source["failedCount"];
	        this.durationMs = source["durationMs"];
	        this.items = this.convertValues(source["items"], BackupItemResult);
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
	export class DeleteResult {
	    fileId: number;
	    status: string;
	    deletedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new DeleteResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fileId = source["fileId"];
	        this.status = source["status"];
	        this.deletedAt = source["deletedAt"];
	    }
	}
	export class FileMetadata {
	    fileId: number;
	    fileName: string;
	    path: string;
	    sizeBytes: number;
	    mimeType: string;
	    modifiedAt: string;
	    aiStatus: string;
	    description?: string;
	    backupJobId?: number;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new FileMetadata(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fileId = source["fileId"];
	        this.fileName = source["fileName"];
	        this.path = source["path"];
	        this.sizeBytes = source["sizeBytes"];
	        this.mimeType = source["mimeType"];
	        this.modifiedAt = source["modifiedAt"];
	        this.aiStatus = source["aiStatus"];
	        this.description = source["description"];
	        this.backupJobId = source["backupJobId"];
	        this.status = source["status"];
	    }
	}

}

