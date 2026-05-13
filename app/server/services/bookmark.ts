import fs from 'fs';
import path from 'path';
import crypto from 'crypto';
import config from '../utils/config';
import Logger from '../utils/logger';

const logger = Logger.child({ module: 'Bookmark' });

export interface Bookmark {
    id: string;
    name: string;
    path: string;
    isDocker?: boolean;
    createdAt: string;
}

interface BookmarkConfig {
    bookmarks: Bookmark[];
}

const CONFIG_FILENAME = 'bookmarks.json';
const MAX_BOOKMARKS = 50;

function getConfigPath(): string {
    return path.join(config.dataDir, 'config', CONFIG_FILENAME);
}

async function ensureDataDir(): Promise<void> {
    try {
        await fs.promises.mkdir(path.join(config.dataDir, 'config'), { recursive: true, mode: 0o700 });
    } catch (e) {
        if ((e as NodeJS.ErrnoException).code !== 'EEXIST') throw e;
    }
}

export async function loadBookmarks(): Promise<Bookmark[]> {
    const configPath = getConfigPath();
    try {
        const content = await fs.promises.readFile(configPath, 'utf8');
        const data = JSON.parse(content) as BookmarkConfig;
        return Array.isArray(data.bookmarks) ? data.bookmarks : [];
    } catch (e) {
        if ((e as NodeJS.ErrnoException).code === 'ENOENT') {
            return [];
        }
        logger.error({ err: e }, '加载书签配置失败');
        return [];
    }
}

async function saveBookmarks(bookmarks: Bookmark[]): Promise<void> {
    await ensureDataDir();
    const configPath = getConfigPath();
    const tmpPath = configPath + '.tmp';
    const content = JSON.stringify({ bookmarks }, null, 2);
    await fs.promises.writeFile(tmpPath, content, { mode: 0o600 });
    await fs.promises.rename(tmpPath, configPath);
}

export async function addBookmark(name: string, bookmarkPath: string, isDocker?: boolean): Promise<Bookmark> {
    const bookmarks = await loadBookmarks();
    if (bookmarks.length >= MAX_BOOKMARKS) {
        throw new Error(`书签数量已达上限 ${MAX_BOOKMARKS}`);
    }

    const bookmark: Bookmark = {
        id: crypto.randomUUID(),
        name,
        path: bookmarkPath,
        isDocker: isDocker || false,
        createdAt: new Date().toISOString()
    };

    bookmarks.push(bookmark);
    await saveBookmarks(bookmarks);
    return bookmark;
}

export async function deleteBookmark(id: string): Promise<boolean> {
    const bookmarks = await loadBookmarks();
    const index = bookmarks.findIndex(b => b.id === id);
    if (index === -1) return false;
    bookmarks.splice(index, 1);
    await saveBookmarks(bookmarks);
    return true;
}

export async function updateBookmark(id: string, name: string): Promise<Bookmark | null> {
    const bookmarks = await loadBookmarks();
    const bookmark = bookmarks.find(b => b.id === id);
    if (!bookmark) return null;
    bookmark.name = name;
    await saveBookmarks(bookmarks);
    return bookmark;
}
