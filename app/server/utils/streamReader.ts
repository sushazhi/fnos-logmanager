import fs from 'fs';
import path from 'path';
import { createReadStream } from 'fs';
import { createInterface } from 'readline';
import Logger from './logger';

const logger = Logger.child({ module: 'LogStreamReader' });

/**
 * 流式日志读取选项
 */
export interface StreamReadOptions {
  maxLines?: number; // 最大行数
  maxSize?: number; // 最大字节数
  encoding?: BufferEncoding; // 编码
  startLine?: number; // 起始行
  reverse?: boolean; // 是否倒序读取
}

/**
 * 流式日志读取结果
 */
export interface StreamReadResult {
  content: string;
  lines: number;
  size: number;
  truncated: boolean;
}

/**
 * 流式读取日志文件
 */
export async function readLogStream(
  filePath: string,
  options: StreamReadOptions = {}
): Promise<StreamReadResult> {
  const {
    maxLines = 10000,
    maxSize = 10 * 1024 * 1024, // 10MB
    encoding = 'utf-8',
    startLine = 0,
    reverse = false
  } = options;

  // 验证文件路径
  if (!filePath || !path.isAbsolute(filePath)) {
    throw new Error('无效的文件路径');
  }

  // 检查文件是否存在
  if (!fs.existsSync(filePath)) {
    throw new Error('文件不存在');
  }

  // 获取文件状态
  const stats = fs.statSync(filePath);
  
  if (stats.size === 0) {
    return {
      content: '',
      lines: 0,
      size: 0,
      truncated: false
    };
  }

  // 如果文件大小超过最大限制，使用尾部读取
  if (stats.size > maxSize) {
    return readTail(filePath, maxSize, encoding);
  }

  // 正常流式读取
  return readNormal(filePath, {
    maxLines,
    encoding,
    startLine,
    reverse
  });
}

/**
 * 正常流式读取
 */
async function readNormal(
  filePath: string,
  options: {
    maxLines: number;
    encoding: BufferEncoding;
    startLine: number;
    reverse: boolean;
  }
): Promise<StreamReadResult> {
  const { maxLines, encoding, startLine, reverse } = options;

  const stream = createReadStream(filePath, { encoding });
  const rl = createInterface({ input: stream, crlfDelay: Infinity });

  const lines: string[] = [];
  let currentLine = 0;
  let totalSize = 0;

  try {
    for await (const line of rl) {
      currentLine++;

      // 跳过起始行之前的行
      if (currentLine <= startLine) {
        continue;
      }

      lines.push(line);
      totalSize += Buffer.byteLength(line, encoding);

      // 达到最大行数限制
      if (lines.length >= maxLines) {
        break;
      }
    }

    // 如果需要倒序
    if (reverse) {
      lines.reverse();
    }

    return {
      content: lines.join('\n'),
      lines: lines.length,
      size: totalSize,
      truncated: false
    };
  } catch (err) {
    logger.error({ err, filePath }, '流式读取失败');
    throw err;
  } finally {
    rl.close();
    stream.destroy();
  }
}

/**
 * 尾部读取（用于大文件）
 */
async function readTail(
  filePath: string,
  maxSize: number,
  encoding: BufferEncoding
): Promise<StreamReadResult> {
  const stats = fs.statSync(filePath);
  const start = Math.max(0, stats.size - maxSize);

  const stream = createReadStream(filePath, {
    encoding,
    start
  });

  const chunks: string[] = [];

  return new Promise((resolve, reject) => {
    stream.on('data', (chunk) => {
      chunks.push(chunk.toString());
    });

    stream.on('end', () => {
      const content = chunks.join('');
      const lines = content.split('\n');

      // 移除第一行（可能不完整）
      if (start > 0 && lines.length > 1) {
        lines.shift();
      }

      resolve({
        content: lines.join('\n'),
        lines: lines.length,
        size: Buffer.byteLength(content, encoding),
        truncated: true
      });
    });

    stream.on('error', (err) => {
      logger.error({ err, filePath }, '尾部读取失败');
      reject(err);
    });
  });
}

/**
 * 搜索日志内容
 */
export async function searchLogContent(
  filePath: string,
  pattern: string | RegExp,
  options: {
    maxResults?: number;
    contextLines?: number; // 上下文行数
    encoding?: BufferEncoding;
  } = {}
): Promise<Array<{ line: number; content: string; context?: string[] }>> {
  const {
    maxResults = 100,
    contextLines = 0,
    encoding = 'utf-8'
  } = options;

  const stream = createReadStream(filePath, { encoding });
  const rl = createInterface({ input: stream, crlfDelay: Infinity });

  const results: Array<{ line: number; content: string; context?: string[] }> = [];
  const lineBuffer: string[] = [];
  let currentLine = 0;

  const regex = typeof pattern === 'string' ? new RegExp(pattern, 'gi') : pattern;

  try {
    for await (const line of rl) {
      currentLine++;
      lineBuffer.push(line);

      // 保持上下文窗口
      if (lineBuffer.length > contextLines * 2 + 1) {
        lineBuffer.shift();
      }

      if (regex.test(line)) {
        const result: any = {
          line: currentLine,
          content: line
        };

        // 添加上下文
        if (contextLines > 0) {
          const contextStart = Math.max(0, lineBuffer.length - contextLines - 1);
          result.context = lineBuffer.slice(contextStart, lineBuffer.length - 1);
        }

        results.push(result);

        if (results.length >= maxResults) {
          break;
        }
      }
    }

    return results;
  } finally {
    rl.close();
    stream.destroy();
  }
}
