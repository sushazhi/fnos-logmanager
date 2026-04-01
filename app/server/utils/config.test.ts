import config from './config';

describe('Config', () => {
    describe('default values', () => {
        it('should have default port', () => {
            expect(config.port).toBeDefined();
            expect(typeof config.port).toBe('number');
            expect(config.port).toBeGreaterThan(0);
            expect(config.port).toBeLessThan(65536);
        });

        it('should have default dataDir', () => {
            expect(config.dataDir).toBeDefined();
            expect(typeof config.dataDir).toBe('string');
            expect(config.dataDir.length).toBeGreaterThan(0);
        });

        it('should have sessionExpiry', () => {
            expect(config.sessionExpiry).toBeDefined();
            expect(config.sessionExpiry).toBeGreaterThan(0);
        });
    });

    describe('rate limit config', () => {
        it('should have rateLimit settings', () => {
            expect(config.rateLimit).toBeDefined();
            expect(config.rateLimit.windowMs).toBeGreaterThan(0);
            expect(config.rateLimit.maxRequests).toBeGreaterThan(0);
        });
    });

    describe('auth config', () => {
        it('should have auth settings', () => {
            expect(config.auth).toBeDefined();
            expect(typeof config.auth.allowQueryToken).toBe('boolean');
            expect(Array.isArray(config.auth.queryTokenPaths)).toBe(true);
        });

        it('should have login settings', () => {
            expect(config.login).toBeDefined();
            expect(config.login.maxAttempts).toBeGreaterThan(0);
            expect(config.login.lockoutTime).toBeGreaterThan(0);
        });
    });

    describe('audit config', () => {
        it('should have audit settings', () => {
            expect(config.audit).toBeDefined();
            expect(config.audit.maxLogs).toBeGreaterThan(0);
        });
    });

    describe('csrf config', () => {
        it('should have csrf settings', () => {
            expect(config.csrf).toBeDefined();
            expect(config.csrf.expiry).toBeGreaterThan(0);
        });
    });

    describe('docker config', () => {
        it('should have docker settings', () => {
            expect(config.docker).toBeDefined();
            expect(config.docker.listTimeoutMs).toBeGreaterThan(0);
            expect(config.docker.logsTimeoutMs).toBeGreaterThan(0);
            expect(config.docker.maxLogLines).toBeGreaterThan(0);
            expect(config.docker.maxOutputBytes).toBeGreaterThan(0);
        });
    });

    describe('logDirs', () => {
        it('should have logDirs array', () => {
            expect(Array.isArray(config.logDirs)).toBe(true);
            expect(config.logDirs.length).toBeGreaterThan(0);
        });

        it('should contain valid paths', () => {
            for (const dir of config.logDirs) {
                expect(typeof dir).toBe('string');
                expect(dir.length).toBeGreaterThan(0);
            }
        });
    });

    describe('sensitivePatterns', () => {
        it('should have sensitivePatterns array', () => {
            expect(Array.isArray(config.sensitivePatterns)).toBe(true);
        });

        it('should contain RegExp patterns', () => {
            for (const pattern of config.sensitivePatterns) {
                expect(pattern instanceof RegExp).toBe(true);
            }
        });
    });

    describe('file paths', () => {
        it('should have passwordFile path', () => {
            expect(config.passwordFile).toBeDefined();
            expect(typeof config.passwordFile).toBe('string');
        });

        it('should have auditLogFile path', () => {
            expect(config.auditLogFile).toBeDefined();
            expect(typeof config.auditLogFile).toBe('string');
        });

        it('should have initTimestampFile path', () => {
            expect(config.initTimestampFile).toBeDefined();
            expect(typeof config.initTimestampFile).toBe('string');
        });
    });

    describe('eventLogger config', () => {
        it('should have eventLogger settings', () => {
            expect(config.eventLogger).toBeDefined();
            expect(typeof config.eventLogger.dbPath).toBe('string');
            expect(typeof config.eventLogger.enabled).toBe('boolean');
            expect(config.eventLogger.checkInterval).toBeGreaterThan(0);
        });
    });

    describe('update config', () => {
        it('should have update settings', () => {
            expect(config.update).toBeDefined();
            expect(config.update.checkCacheMs).toBeGreaterThan(0);
            expect(config.update.downloadTimeoutMs).toBeGreaterThan(0);
            expect(config.update.maxDownloadBytes).toBeGreaterThan(0);
            expect(config.update.maxRedirects).toBeGreaterThan(0);
        });

        it('should have allowedHosts', () => {
            expect(Array.isArray(config.update.allowedHosts)).toBe(true);
            expect(config.update.allowedHosts.length).toBeGreaterThan(0);
        });
    });

    describe('backup config', () => {
        it('should have backup settings', () => {
            expect(config.backup).toBeDefined();
            expect(typeof config.backup.baseDir).toBe('string');
            expect(config.backup.maxFiles).toBeGreaterThan(0);
            expect(config.backup.maxFileSizeBytes).toBeGreaterThan(0);
            expect(config.backup.maxTotalBytes).toBeGreaterThan(0);
        });
    });
});
