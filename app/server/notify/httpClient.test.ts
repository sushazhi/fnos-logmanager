/**
 * HTTP Client 测试
 */
import { httpRequest, httpClient, $ } from '../httpClient';

// Mock undici
jest.mock('undici', () => ({
    request: jest.fn(),
    FormData: jest.fn()
}));

describe('HttpClient', () => {
    describe('httpClient object', () => {
        it('should have request method', () => {
            expect(typeof httpClient.request).toBe('function');
        });

        it('should have post method', () => {
            expect(typeof httpClient.post).toBe('function');
        });

        it('should have get method', () => {
            expect(typeof httpClient.get).toBe('function');
        });
    });

    describe('$ object', () => {
        it('should have post method', () => {
            expect(typeof $.post).toBe('function');
        });

        it('should have get method', () => {
            expect(typeof $.get).toBe('function');
        });

        it('should have logErr method', () => {
            expect(typeof $.logErr).toBe('function');
        });

        it('logErr should not throw', () => {
            expect(() => $.logErr(new Error('test'))).not.toThrow();
        });
    });
});
