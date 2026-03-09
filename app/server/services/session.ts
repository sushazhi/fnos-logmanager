import crypto from 'crypto';
import config from '../utils/config';
import { Session, CSRFToken } from '../types';

const sessions = new Map<string, Session>();
const csrfTokens = new Map<string, CSRFToken>();

export function generateToken(): string {
    return crypto.randomBytes(32).toString('hex');
}

export function createSession(username: string): string {
    const token = generateToken();
    sessions.set(token, {
        username: username,
        createdAt: Date.now(),
        lastAccess: Date.now()
    });
    return token;
}

export function validateSession(token: string | undefined): boolean {
    if (!token) return false;
    const session = sessions.get(token);
    if (!session) return false;

    if (Date.now() - session.lastAccess > config.sessionExpiry) {
        sessions.delete(token);
        csrfTokens.delete(token);
        return false;
    }

    session.lastAccess = Date.now();
    return true;
}

export function getSession(token: string): Session | null {
    return sessions.get(token) || null;
}

export function deleteSession(token: string): void {
    sessions.delete(token);
    csrfTokens.delete(token);
}

export function cleanExpiredSessions(): void {
    const now = Date.now();
    for (const [token, session] of sessions.entries()) {
        if (now - session.lastAccess > config.sessionExpiry) {
            sessions.delete(token);
            csrfTokens.delete(token);
        }
    }
}

export function getCSRFToken(sessionToken: string): string | null {
    if (sessionToken && sessions.has(sessionToken)) {
        if (csrfTokens.has(sessionToken)) {
            const stored = csrfTokens.get(sessionToken);
            if (stored && Date.now() - stored.createdAt < config.csrf.expiry) {
                return stored.token;
            }
        }
        const csrfToken = generateToken();
        csrfTokens.set(sessionToken, { token: csrfToken, createdAt: Date.now() });
        return csrfToken;
    }
    return null;
}

export function validateCSRFToken(sessionToken: string, csrfToken: string): boolean {
    if (!sessionToken || !csrfToken) return false;
    const stored = csrfTokens.get(sessionToken);
    if (!stored) return false;
    if (Date.now() - stored.createdAt > config.csrf.expiry) {
        csrfTokens.delete(sessionToken);
        return false;
    }
    return stored.token === csrfToken;
}

export function cleanExpiredCSRFTokens(): void {
    const now = Date.now();
    for (const [sessionToken, data] of csrfTokens.entries()) {
        if (now - data.createdAt > config.csrf.expiry) {
            csrfTokens.delete(sessionToken);
        }
    }
}

setInterval(cleanExpiredSessions, 3600000);
setInterval(cleanExpiredCSRFTokens, 3600000);
