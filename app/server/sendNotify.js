const querystring = require('node:querystring');
const { request: undiciRequest, FormData } = require('undici');
const timeout = 15000;

async function request(url, options = {}) {
  const { json, form, body, headers = {}, ...rest } = options;

  const finalHeaders = { ...headers };
  let finalBody = body;

  if (json) {
    finalHeaders['content-type'] = 'application/json';
    finalBody = JSON.stringify(json);
  } else if (form) {
    finalBody = form;
    delete finalHeaders['content-type'];
  }

  return undiciRequest(url, {
    headers: finalHeaders,
    body: finalBody,
    ...rest,
  });
}

function post(url, options = {}) {
  return request(url, { ...options, method: 'POST' });
}

function get(url, options = {}) {
  return request(url, { ...options, method: 'GET' });
}

const httpClient = {
  request,
  post,
  get,
};

const push_config = {
  BARK_PUSH: '', // bark IP 或设备码，例：https://api.day.app/DxHcxxxxxRxxxxxxcm/
  BARK_ARCHIVE: '', // bark 推送是否存档
  BARK_GROUP: '', // bark 推送分组
  BARK_SOUND: '', // bark 推送声音
  BARK_ICON: '', // bark 推送图标
  BARK_LEVEL: '', // bark 推送时效性
  BARK_URL: '', // bark 推送跳转URL

  DD_BOT_SECRET: '', // 钉钉机器人的 DD_BOT_SECRET
  DD_BOT_TOKEN: '', // 钉钉机器人的 DD_BOT_TOKEN

  FSKEY: '', // 飞书机器人的 FSKEY（自定义机器人Webhook Key）
  FSSECRET: '', // 飞书机器人的 FSSECRET（自定义机器人签名密钥）
  FEISHU_APP_ID: '', // 飞书企业自建应用的 App ID
  FEISHU_APP_SECRET: '', // 飞书企业自建应用的 App Secret
  FEISHU_USER_ID: '', // 飞书用户ID（使用user_id类型）

  GOTIFY_URL: '', // gotify地址,如https://push.example.de:8080
  GOTIFY_TOKEN: '', // gotify的消息应用token
  GOTIFY_PRIORITY: 0, // 推送消息优先级,默认为0

  IGOT_PUSH_KEY: '', // iGot 聚合推送的 IGOT_PUSH_KEY，例如：https://push.hellyw.com/XXXXXXXX

  PUSH_KEY: '', // server 酱的 PUSH_KEY，兼容旧版与 Turbo 版

  DEER_KEY: '', // PushDeer 的 PUSHDEER_KEY
  DEER_URL: '', // PushDeer 的 PUSHDEER_URL

  CHAT_URL: '', // synology chat url
  CHAT_TOKEN: '', // synology chat token

  // 官方文档：https://www.pushplus.plus/
  PUSH_PLUS_TOKEN: '', // pushplus 推送的用户令牌
  PUSH_PLUS_USER: '', // pushplus 推送的群组编码
  PUSH_PLUS_TEMPLATE: 'html', // pushplus 发送模板，支持html,txt,json,markdown,cloudMonitor,jenkins,route,pay
  PUSH_PLUS_CHANNEL: 'wechat', // pushplus 发送渠道，支持wechat,webhook,cp,mail,sms
  PUSH_PLUS_WEBHOOK: '', // pushplus webhook编码，可在pushplus公众号上扩展配置出更多渠道
  PUSH_PLUS_CALLBACKURL: '', // pushplus 发送结果回调地址，会把推送最终结果通知到这个地址上
  PUSH_PLUS_TO: '', // pushplus 好友令牌，微信公众号渠道填写好友令牌，企业微信渠道填写企业微信用户id

  // 微加机器人，官方网站：https://www.weplusbot.com/
  WE_PLUS_BOT_TOKEN: '', // 微加机器人的用户令牌
  WE_PLUS_BOT_RECEIVER: '', // 微加机器人的消息接收人
  WE_PLUS_BOT_VERSION: 'pro', //微加机器人调用版本，pro和personal；为空默认使用pro(专业版)，个人版填写：personal

  QMSG_KEY: '', // qmsg 酱的 QMSG_KEY
  QMSG_TYPE: '', // qmsg 酱的 QMSG_TYPE

  QYWX_ORIGIN: 'https://qyapi.weixin.qq.com', // 企业微信代理地址

  /*
    此处填你企业微信应用消息的值(详见文档 https://work.weixin.qq.com/api/doc/90000/90135/90236)
    环境变量名 QYWX_AM依次填入 corpid,corpsecret,touser(注:多个成员ID使用|隔开),agentid,消息类型(选填,不填默认文本消息类型)
    注意用,号隔开(英文输入法的逗号)，例如：wwcff56746d9adwers,B-791548lnzXBE6_BWfxdf3kSTMJr9vFEPKAbh6WERQ,mingcheng,1000001,2COXgjH2UIfERF2zxrtUOKgQ9XklUqMdGSWLBoW_lSDAdafat
    可选推送消息类型(推荐使用图文消息（mpnews）):
    - 文本卡片消息: 0 (数字零)
    - 文本消息: 1 (数字一)
    - 图文消息（mpnews）: 素材库图片id, 可查看此教程(http://note.youdao.com/s/HMiudGkb)或者(https://note.youdao.com/ynoteshare1/index.html?id=1a0c8aff284ad28cbd011b29b3ad0191&type=note)
  */
  QYWX_AM: '', // 企业微信应用

  QYWX_KEY: '', // 企业微信机器人的 webhook(详见文档 https://work.weixin.qq.com/api/doc/90000/90136/91770)，例如：693a91f6-7xxx-4bc4-97a0-0ec2sifa5aaa

  // 企业微信智能机器人（长连接模式）
  WECHAT_BOT_ID: '', // 企业微信智能机器人 ID
  WECHAT_BOT_SECRET: '', // 企业微信智能机器人 Secret
  WECHAT_BOT_CHAT_ID: '', // 默认发送目标，支持 user:xxx 或 group:xxx 格式
  WECHAT_BOT_WS_URL: 'wss://openws.work.weixin.qq.com', // WebSocket 地址

  TG_BOT_TOKEN: '', // tg 机器人的 TG_BOT_TOKEN
  TG_USER_ID: '', // tg 机器人的 TG_USER_ID，例：1434078534
  TG_API_HOST: 'https://api.telegram.org', // tg 代理 api
  TG_PROXY_AUTH: '', // tg 代理认证参数
  TG_PROXY_HOST: '', // tg 机器人的 TG_PROXY_HOST
  TG_PROXY_PORT: '', // tg 机器人的 TG_PROXY_PORT

  AIBOTK_KEY: '', // 智能微秘书 个人中心的apikey 文档地址：http://wechat.aibotk.com/docs/about
  AIBOTK_TYPE: '', // 智能微秘书 发送目标 room 或 contact
  AIBOTK_NAME: '', // 智能微秘书  发送群名 或者好友昵称和type要对应好

  PUSHME_KEY: '', // 官方文档：https://push.i-i.me，PushMe 酱的 PUSHME_KEY

  // CHRONOCAT API https://chronocat.vercel.app/install/docker/official/
  CHRONOCAT_QQ: '', // 个人: user_id=个人QQ 群则填入 group_id=QQ群 多个用英文;隔开同时支持个人和群
  CHRONOCAT_TOKEN: '', // 填写在CHRONOCAT文件生成的访问密钥
  CHRONOCAT_URL: '', // Red 协议连接地址 例： http://127.0.0.1:16530

  WEBHOOK_URL: '', // 自定义通知 请求地址
  WEBHOOK_BODY: '', // 自定义通知 请求体
  WEBHOOK_HEADERS: '', // 自定义通知 请求头
  WEBHOOK_METHOD: '', // 自定义通知 请求方法
  WEBHOOK_CONTENT_TYPE: '', // 自定义通知 content-type

  NTFY_URL: '', // ntfy地址,如https://ntfy.sh,默认为https://ntfy.sh
  NTFY_TOPIC: '', // ntfy的消息应用topic
  NTFY_PRIORITY: '3', // 推送消息优先级,默认为3
  NTFY_TOKEN: '', // 推送token,可选
  NTFY_USERNAME: '', // 推送用户名称,可选
  NTFY_PASSWORD: '', // 推送用户密码,可选
  NTFY_ACTIONS: '', // 推送用户动作,可选

  // 官方文档: https://wxpusher.zjiecode.com/docs/
  // 管理后台: https://wxpusher.zjiecode.com/admin/
  WXPUSHER_APP_TOKEN: '', // wxpusher 的 appToken
  WXPUSHER_TOPIC_IDS: '', // wxpusher 的 主题ID，多个用英文分号;分隔 topic_ids 与 uids 至少配置一个才行
  WXPUSHER_UIDS: '', // wxpusher 的 用户ID，多个用英文分号;分隔 topic_ids 与 uids 至少配置一个才行

  // QQ机器人配置（基于QQ开放平台API）
  QQ_APP_ID: '', // QQ机器人 AppID
  QQ_APP_SECRET: '', // QQ机器人 AppSecret
  QQ_OPENID: '', // 默认接收者 openid（单聊）
  QQ_GROUP_OPENID: '', // 默认群组 openid（群聊，与 QQ_OPENID 二选一）
};

for (const key in push_config) {
  const v = process.env[key];
  if (v) {
    push_config[key] = v;
  }
}

const $ = {
  post: (params, callback) => {
    const { url, ...others } = params;
    httpClient.post(url, others).then(
      async (res) => {
        let body = await res.body.text();
        try {
          body = JSON.parse(body);
        } catch (error) {}
        callback(null, res, body);
      },
      (err) => {
        callback(err?.response?.body || err);
      },
    );
  },
  get: (params, callback) => {
    const { url, ...others } = params;
    httpClient.get(url, others).then(
      async (res) => {
        let body = await res.body.text();
        try {
          body = JSON.parse(body);
        } catch (error) {}
        callback(null, res, body);
      },
      (err) => {
        callback(err?.response?.body || err);
      },
    );
  },
  logErr: console.log,
};

function gotifyNotify(text, desp) {
  return new Promise((resolve) => {
    const { GOTIFY_URL, GOTIFY_TOKEN, GOTIFY_PRIORITY } = push_config;
    if (GOTIFY_URL && GOTIFY_TOKEN) {
      const options = {
        url: `${GOTIFY_URL}/message?token=${GOTIFY_TOKEN}`,
        body: `title=${encodeURIComponent(text)}&message=${encodeURIComponent(
          desp,
        )}&priority=${GOTIFY_PRIORITY}`,
        headers: {
          'Content-Type': 'application/x-www-form-urlencoded',
        },
      };
      $.post(options, (err, resp, data) => {
        try {
          if (err) {
            console.log('Gotify 发送通知调用API失败😞\n', err);
          } else {
            if (data.id) {
              console.log('Gotify 发送通知消息成功🎉\n');
            } else {
              console.log(`Gotify 发送通知调用API失败😞 ${data.message}\n`);
            }
          }
        } catch (e) {
          $.logErr(e, resp);
        } finally {
          resolve();
        }
      });
    } else {
      resolve();
    }
  });
}

function serverNotify(text, desp) {
  return new Promise((resolve) => {
    const { PUSH_KEY } = push_config;
    if (PUSH_KEY) {
      // 微信server酱推送通知一个\n不会换行，需要两个\n才能换行，故做此替换
      desp = desp.replace(/[\n\r]/g, '\n\n');

      const matchResult = PUSH_KEY.match(/^sctp(\d+)t/i);
      const options = {
        url:
          matchResult && matchResult[1]
            ? `https://${matchResult[1]}.push.ft07.com/send/${PUSH_KEY}.send`
            : `https://sctapi.ftqq.com/${PUSH_KEY}.send`,
        body: `text=${encodeURIComponent(text)}&desp=${encodeURIComponent(
          desp,
        )}`,
        headers: {
          'Content-Type': 'application/x-www-form-urlencoded',
        },
        timeout,
      };
      $.post(options, (err, resp, data) => {
        try {
          if (err) {
            console.log('Server 酱发送通知调用API失败😞\n', err);
          } else {
            // server酱和Server酱·Turbo版的返回json格式不太一样
            if (data.errno === 0 || data.data.errno === 0) {
              console.log('Server 酱发送通知消息成功🎉\n');
            } else if (data.errno === 1024) {
              // 一分钟内发送相同的内容会触发
              console.log(`Server 酱发送通知消息异常 ${data.errmsg}\n`);
            } else {
              console.log(`Server 酱发送通知消息异常 ${JSON.stringify(data)}`);
            }
          }
        } catch (e) {
          $.logErr(e, resp);
        } finally {
          resolve(data);
        }
      });
    } else {
      resolve();
    }
  });
}

function pushDeerNotify(text, desp) {
  return new Promise((resolve) => {
    const { DEER_KEY, DEER_URL } = push_config;
    if (DEER_KEY) {
      // PushDeer 建议对消息内容进行 urlencode
      desp = encodeURI(desp);
      const options = {
        url: DEER_URL || `https://api2.pushdeer.com/message/push`,
        body: `pushkey=${DEER_KEY}&text=${text}&desp=${desp}&type=markdown`,
        headers: {
          'Content-Type': 'application/x-www-form-urlencoded',
        },
        timeout,
      };
      $.post(options, (err, resp, data) => {
        try {
          if (err) {
            console.log('PushDeer 通知调用API失败😞\n', err);
          } else {
            // 通过返回的result的长度来判断是否成功
            if (
              data.content.result.length !== undefined &&
              data.content.result.length > 0
            ) {
              console.log('PushDeer 发送通知消息成功🎉\n');
            } else {
              console.log(
                `PushDeer 发送通知消息异常😞 ${JSON.stringify(data)}`,
              );
            }
          }
        } catch (e) {
          $.logErr(e, resp);
        } finally {
          resolve(data);
        }
      });
    } else {
      resolve();
    }
  });
}

function chatNotify(text, desp) {
  return new Promise((resolve) => {
    const { CHAT_URL, CHAT_TOKEN } = push_config;
    if (CHAT_URL && CHAT_TOKEN) {
      // 对消息内容进行 urlencode
      desp = encodeURI(desp);
      const options = {
        url: `${CHAT_URL}${CHAT_TOKEN}`,
        body: `payload={"text":"${text}\n${desp}"}`,
        headers: {
          'Content-Type': 'application/x-www-form-urlencoded',
        },
      };
      $.post(options, (err, resp, data) => {
        try {
          if (err) {
            console.log('Chat 发送通知调用API失败😞\n', err);
          } else {
            if (data.success) {
              console.log('Chat 发送通知消息成功🎉\n');
            } else {
              console.log(`Chat 发送通知消息异常 ${JSON.stringify(data)}`);
            }
          }
        } catch (e) {
          $.logErr(e);
        } finally {
          resolve(data);
        }
      });
    } else {
      resolve();
    }
  });
}

function barkNotify(text, desp, params = {}) {
  return new Promise((resolve) => {
    let {
      BARK_PUSH,
      BARK_ICON,
      BARK_SOUND,
      BARK_GROUP,
      BARK_LEVEL,
      BARK_ARCHIVE,
      BARK_URL,
    } = push_config;
    if (BARK_PUSH) {
      // 兼容BARK本地用户只填写设备码的情况
      if (!BARK_PUSH.startsWith('http')) {
        BARK_PUSH = `https://api.day.app/${BARK_PUSH}`;
      }
      const options = {
        url: `${BARK_PUSH}`,
        json: {
          title: text,
          body: desp,
          icon: BARK_ICON,
          sound: BARK_SOUND,
          group: BARK_GROUP,
          isArchive: BARK_ARCHIVE,
          level: BARK_LEVEL,
          url: BARK_URL,
          ...params,
        },
        headers: {
          'Content-Type': 'application/json',
        },
        timeout,
      };
      $.post(options, (err, resp, data) => {
        try {
          if (err) {
            console.log('Bark APP 发送通知调用API失败😞\n', err);
          } else {
            if (data.code === 200) {
              console.log('Bark APP 发送通知消息成功🎉\n');
            } else {
              console.log(`Bark APP 发送通知消息异常 ${data.message}\n`);
            }
          }
        } catch (e) {
          $.logErr(e, resp);
        } finally {
          resolve();
        }
      });
    } else {
      resolve();
    }
  });
}

function tgBotNotify(text, desp) {
  return new Promise((resolve) => {
    const {
      TG_BOT_TOKEN,
      TG_USER_ID,
      TG_PROXY_HOST,
      TG_PROXY_PORT,
      TG_API_HOST,
      TG_PROXY_AUTH,
    } = push_config;
    if (TG_BOT_TOKEN && TG_USER_ID) {
      let options = {
        url: `${TG_API_HOST}/bot${TG_BOT_TOKEN}/sendMessage`,
        json: {
          chat_id: `${TG_USER_ID}`,
          text: `${text}\n\n${desp}`,
          disable_web_page_preview: true,
        },
        headers: {
          'Content-Type': 'application/json',
        },
        timeout,
      };
      if (TG_PROXY_HOST && TG_PROXY_PORT) {
        let proxyHost = TG_PROXY_HOST;
        if (TG_PROXY_AUTH && !TG_PROXY_HOST.includes('@')) {
          proxyHost = `${TG_PROXY_AUTH}@${TG_PROXY_HOST}`;
        }
        let agent;
        agent = new ProxyAgent({
          uri: `http://${proxyHost}:${TG_PROXY_PORT}`,
        });
        options.dispatcher = agent;
      }
      $.post(options, (err, resp, data) => {
        try {
          if (err) {
            console.log('Telegram 发送通知消息失败😞\n', err);
          } else {
            if (data.ok) {
              console.log('Telegram 发送通知消息成功🎉。\n');
            } else if (data.error_code === 400) {
              console.log(
                '请主动给bot发送一条消息并检查接收用户ID是否正确。\n',
              );
            } else if (data.error_code === 401) {
              console.log('Telegram bot token 填写错误。\n');
            }
          }
        } catch (e) {
          $.logErr(e, resp);
        } finally {
          resolve(data);
        }
      });
    } else {
      resolve();
    }
  });
}
function ddBotNotify(text, desp) {
  return new Promise((resolve) => {
    const { DD_BOT_TOKEN, DD_BOT_SECRET } = push_config;
    const options = {
      url: `https://oapi.dingtalk.com/robot/send?access_token=${DD_BOT_TOKEN}`,
      json: {
        msgtype: 'text',
        text: {
          content: `${text}\n\n${desp}`,
        },
      },
      headers: {
        'Content-Type': 'application/json',
      },
      timeout,
    };
    if (DD_BOT_TOKEN && DD_BOT_SECRET) {
      const crypto = require('crypto');
      const dateNow = Date.now();
      const hmac = crypto.createHmac('sha256', DD_BOT_SECRET);
      hmac.update(`${dateNow}\n${DD_BOT_SECRET}`);
      const result = encodeURIComponent(hmac.digest('base64'));
      options.url = `${options.url}&timestamp=${dateNow}&sign=${result}`;
      $.post(options, (err, resp, data) => {
        try {
          if (err) {
            console.log('钉钉发送通知消息失败😞\n', err);
          } else {
            if (data.errcode === 0) {
              console.log('钉钉发送通知消息成功🎉\n');
            } else {
              console.log(`钉钉发送通知消息异常 ${data.errmsg}\n`);
            }
          }
        } catch (e) {
          $.logErr(e, resp);
        } finally {
          resolve(data);
        }
      });
    } else if (DD_BOT_TOKEN) {
      $.post(options, (err, resp, data) => {
        try {
          if (err) {
            console.log('钉钉发送通知消息失败😞\n', err);
          } else {
            if (data.errcode === 0) {
              console.log('钉钉发送通知消息成功🎉\n');
            } else {
              console.log(`钉钉发送通知消息异常 ${data.errmsg}\n`);
            }
          }
        } catch (e) {
          $.logErr(e, resp);
        } finally {
          resolve(data);
        }
      });
    } else {
      resolve();
    }
  });
}

function qywxBotNotify(text, desp) {
  return new Promise((resolve) => {
    const { QYWX_ORIGIN, QYWX_KEY } = push_config;
    const options = {
      url: `${QYWX_ORIGIN}/cgi-bin/webhook/send?key=${QYWX_KEY}`,
      json: {
        msgtype: 'text',
        text: {
          content: `${text}\n\n${desp}`,
        },
      },
      headers: {
        'Content-Type': 'application/json',
      },
      timeout,
    };
    if (QYWX_KEY) {
      $.post(options, (err, resp, data) => {
        try {
          if (err) {
            console.log('企业微信发送通知消息失败😞\n', err);
          } else {
            if (data.errcode === 0) {
              console.log('企业微信发送通知消息成功🎉。\n');
            } else {
              console.log(`企业微信发送通知消息异常 ${data.errmsg}\n`);
            }
          }
        } catch (e) {
          $.logErr(e, resp);
        } finally {
          resolve(data);
        }
      });
    } else {
      resolve();
    }
  });
}

function ChangeUserId(desp) {
  const { QYWX_AM } = push_config;
  const QYWX_AM_AY = QYWX_AM.split(',');
  if (QYWX_AM_AY[2]) {
    const userIdTmp = QYWX_AM_AY[2].split('|');
    let userId = '';
    for (let i = 0; i < userIdTmp.length; i++) {
      const count = '账号' + (i + 1);
      const count2 = '签到号 ' + (i + 1);
      if (desp.match(count2)) {
        userId = userIdTmp[i];
      }
    }
    if (!userId) userId = QYWX_AM_AY[2];
    return userId;
  } else {
    return '@all';
  }
}

async function qywxamNotify(text, desp) {
  const MAX_LENGTH = 900;
  if (desp.length > MAX_LENGTH) {
    let d = desp.substr(0, MAX_LENGTH) + '\n==More==';
    await do_qywxamNotify(text, d);
    await qywxamNotify(text, desp.substr(MAX_LENGTH));
  } else {
    return await do_qywxamNotify(text, desp);
  }
}

function do_qywxamNotify(text, desp) {
  return new Promise((resolve) => {
    const { QYWX_AM, QYWX_ORIGIN } = push_config;
    if (QYWX_AM) {
      const QYWX_AM_AY = QYWX_AM.split(',');
      const options_accesstoken = {
        url: `${QYWX_ORIGIN}/cgi-bin/gettoken`,
        json: {
          corpid: `${QYWX_AM_AY[0]}`,
          corpsecret: `${QYWX_AM_AY[1]}`,
        },
        headers: {
          'Content-Type': 'application/json',
        },
        timeout,
      };
      $.post(options_accesstoken, (err, resp, json) => {
        let html = desp.replace(/\n/g, '<br/>');
        let accesstoken = json.access_token;
        let options;

        switch (QYWX_AM_AY[4]) {
          case '0':
            options = {
              msgtype: 'textcard',
              textcard: {
                title: `${text}`,
                description: `${desp}`,
                url: 'https://github.com/whyour/qinglong',
                btntxt: '更多',
              },
            };
            break;

          case '1':
            options = {
              msgtype: 'text',
              text: {
                content: `${text}\n\n${desp}`,
              },
            };
            break;

          default:
            options = {
              msgtype: 'mpnews',
              mpnews: {
                articles: [
                  {
                    title: `${text}`,
                    thumb_media_id: `${QYWX_AM_AY[4]}`,
                    author: `智能助手`,
                    content_source_url: ``,
                    content: `${html}`,
                    digest: `${desp}`,
                  },
                ],
              },
            };
        }
        if (!QYWX_AM_AY[4]) {
          // 如不提供第四个参数,则默认进行文本消息类型推送
          options = {
            msgtype: 'text',
            text: {
              content: `${text}\n\n${desp}`,
            },
          };
        }
        options = {
          url: `${QYWX_ORIGIN}/cgi-bin/message/send?access_token=${accesstoken}`,
          json: {
            touser: `${ChangeUserId(desp)}`,
            agentid: `${QYWX_AM_AY[3]}`,
            safe: '0',
            ...options,
          },
          headers: {
            'Content-Type': 'application/json',
          },
        };

        $.post(options, (err, resp, data) => {
          try {
            if (err) {
              console.log(
                '成员ID:' +
                  ChangeUserId(desp) +
                  '企业微信应用消息发送通知消息失败😞\n',
                err,
              );
            } else {
              if (data.errcode === 0) {
                console.log(
                  '成员ID:' +
                    ChangeUserId(desp) +
                    '企业微信应用消息发送通知消息成功🎉。\n',
                );
              } else {
                console.log(
                  `企业微信应用消息发送通知消息异常 ${data.errmsg}\n`,
                );
              }
            }
          } catch (e) {
            $.logErr(e, resp);
          } finally {
            resolve(data);
          }
        });
      });
    } else {
      resolve();
    }
  });
}

function iGotNotify(text, desp, params = {}) {
  return new Promise((resolve) => {
    const { IGOT_PUSH_KEY } = push_config;
    if (IGOT_PUSH_KEY) {
      // 校验传入的IGOT_PUSH_KEY是否有效
      const IGOT_PUSH_KEY_REGX = new RegExp('^[a-zA-Z0-9]{24}$');
      if (!IGOT_PUSH_KEY_REGX.test(IGOT_PUSH_KEY)) {
        console.log('您所提供的 IGOT_PUSH_KEY 无效\n');
        resolve();
        return;
      }
      const options = {
        url: `https://push.hellyw.com/${IGOT_PUSH_KEY.toLowerCase()}`,
        body: `title=${text}&content=${desp}&${querystring.stringify(params)}`,
        headers: {
          'Content-Type': 'application/x-www-form-urlencoded',
        },
        timeout,
      };
      $.post(options, (err, resp, data) => {
        try {
          if (err) {
            console.log('IGot 发送通知调用API失败😞\n', err);
          } else {
            if (data.ret === 0) {
              console.log('IGot 发送通知消息成功🎉\n');
            } else {
              console.log(`IGot 发送通知消息异常 ${data.errMsg}\n`);
            }
          }
        } catch (e) {
          $.logErr(e, resp);
        } finally {
          resolve(data);
        }
      });
    } else {
      resolve();
    }
  });
}

function pushPlusNotify(text, desp) {
  return new Promise((resolve) => {
    const {
      PUSH_PLUS_TOKEN,
      PUSH_PLUS_USER,
      PUSH_PLUS_TEMPLATE,
      PUSH_PLUS_CHANNEL,
      PUSH_PLUS_WEBHOOK,
      PUSH_PLUS_CALLBACKURL,
      PUSH_PLUS_TO,
    } = push_config;
    if (PUSH_PLUS_TOKEN) {
      desp = desp.replace(/[\n\r]/g, '<br>'); // 默认为html, 不支持plaintext
      const body = {
        token: `${PUSH_PLUS_TOKEN}`,
        title: `${text}`,
        content: `${desp}`,
        topic: `${PUSH_PLUS_USER}`,
        template: `${PUSH_PLUS_TEMPLATE}`,
        channel: `${PUSH_PLUS_CHANNEL}`,
        webhook: `${PUSH_PLUS_WEBHOOK}`,
        callbackUrl: `${PUSH_PLUS_CALLBACKURL}`,
        to: `${PUSH_PLUS_TO}`,
      };
      const options = {
        url: `https://www.pushplus.plus/send`,
        body: JSON.stringify(body),
        headers: {
          'Content-Type': ' application/json',
        },
        timeout,
      };
      $.post(options, (err, resp, data) => {
        try {
          if (err) {
            console.log(
              `pushplus 发送${
                PUSH_PLUS_USER ? '一对多' : '一对一'
              }通知消息失败😞\n`,
              err,
            );
          } else {
            if (data.code === 200) {
              console.log(
                `pushplus 发送${
                  PUSH_PLUS_USER ? '一对多' : '一对一'
                }通知请求成功🎉，可根据流水号查询推送结果：${
                  data.data
                }\n注意：请求成功并不代表推送成功，如未收到消息，请到pushplus官网使用流水号查询推送最终结果`,
              );
            } else {
              console.log(
                `pushplus 发送${
                  PUSH_PLUS_USER ? '一对多' : '一对一'
                }通知消息异常 ${data.msg}\n`,
              );
            }
          }
        } catch (e) {
          $.logErr(e, resp);
        } finally {
          resolve(data);
        }
      });
    } else {
      resolve();
    }
  });
}

function wePlusBotNotify(text, desp) {
  return new Promise((resolve) => {
    const { WE_PLUS_BOT_TOKEN, WE_PLUS_BOT_RECEIVER, WE_PLUS_BOT_VERSION } =
      push_config;
    if (WE_PLUS_BOT_TOKEN) {
      let template = 'txt';
      if (desp.length > 800) {
        desp = desp.replace(/[\n\r]/g, '<br>');
        template = 'html';
      }
      const body = {
        token: `${WE_PLUS_BOT_TOKEN}`,
        title: `${text}`,
        content: `${desp}`,
        template: `${template}`,
        receiver: `${WE_PLUS_BOT_RECEIVER}`,
        version: `${WE_PLUS_BOT_VERSION}`,
      };
      const options = {
        url: `https://www.weplusbot.com/send`,
        body: JSON.stringify(body),
        headers: {
          'Content-Type': ' application/json',
        },
        timeout,
      };
      $.post(options, (err, resp, data) => {
        try {
          if (err) {
            console.log(`微加机器人发送通知消息失败😞\n`, err);
          } else {
            if (data.code === 200) {
              console.log(`微加机器人发送通知消息完成🎉\n`);
            } else {
              console.log(`微加机器人发送通知消息异常 ${data.msg}\n`);
            }
          }
        } catch (e) {
          $.logErr(e, resp);
        } finally {
          resolve(data);
        }
      });
    } else {
      resolve();
    }
  });
}

function aibotkNotify(text, desp) {
  return new Promise((resolve) => {
    const { AIBOTK_KEY, AIBOTK_TYPE, AIBOTK_NAME } = push_config;
    if (AIBOTK_KEY && AIBOTK_TYPE && AIBOTK_NAME) {
      let json = {};
      let url = '';
      switch (AIBOTK_TYPE) {
        case 'room':
          url = 'https://api-bot.aibotk.com/openapi/v1/chat/room';
          json = {
            apiKey: `${AIBOTK_KEY}`,
            roomName: `${AIBOTK_NAME}`,
            message: {
              type: 1,
              content: `【青龙快讯】\n\n${text}\n${desp}`,
            },
          };
          break;
        case 'contact':
          url = 'https://api-bot.aibotk.com/openapi/v1/chat/contact';
          json = {
            apiKey: `${AIBOTK_KEY}`,
            name: `${AIBOTK_NAME}`,
            message: {
              type: 1,
              content: `【青龙快讯】\n\n${text}\n${desp}`,
            },
          };
          break;
      }
      const options = {
        url: url,
        json,
        headers: {
          'Content-Type': 'application/json',
        },
        timeout,
      };
      $.post(options, (err, resp, data) => {
        try {
          if (err) {
            console.log('智能微秘书发送通知消息失败😞\n', err);
          } else {
            if (data.code === 0) {
              console.log('智能微秘书发送通知消息成功🎉。\n');
            } else {
              console.log(`智能微秘书发送通知消息异常 ${data.error}\n`);
            }
          }
        } catch (e) {
          $.logErr(e, resp);
        } finally {
          resolve(data);
        }
      });
    } else {
      resolve();
    }
  });
}

function fsBotNotify(text, desp) {
  return new Promise((resolve) => {
    const { FSKEY, FSSECRET } = push_config;
    if (FSKEY) {
      const crypto = require('crypto');
      
      // 构建消息内容
      const messageContent = `${text}\n\n${desp}`;
      
      // 根据是否启用签名选择消息类型
      let body;
      
      if (FSSECRET) {
        // 带签名验证的消息 - 使用text类型
        const timestamp = Math.floor(Date.now() / 1000).toString();
        const stringToSign = `${timestamp}\n${FSSECRET}`;
        const hmac = crypto.createHmac('sha256', FSSECRET);
        hmac.update(stringToSign);
        const sign = hmac.digest('base64');
        
        body = {
          msg_type: 'text',
          content: { text: messageContent },
          timestamp: timestamp,
          sign: sign
        };
      } else {
        // 不带签名 - 使用富文本消息
        body = {
          msg_type: 'post',
          content: {
            post: {
              zh_cn: {
                title: text,
                content: [
                  [
                    { tag: 'text', text: desp }
                  ]
                ]
              }
            }
          }
        };
      }

      const options = {
        url: `https://open.feishu.cn/open-apis/bot/v2/hook/${FSKEY}`,
        json: body,
        headers: {
          'Content-Type': 'application/json',
        },
        timeout,
      };
      $.post(options, (err, resp, data) => {
        try {
          if (err) {
            console.log('飞书发送通知调用API失败\n', err);
          } else {
            if (data.StatusCode === 0 || data.code === 0) {
              console.log('飞书发送通知消息成功\n');
            } else {
              console.log(`飞书发送通知消息异常 ${data.msg}\n`);
            }
          }
        } catch (e) {
          $.logErr(e, resp);
        } finally {
          resolve(data);
        }
      });
    } else {
      resolve();
    }
  });
}

// 飞书企业自建应用发送消息
let feishuTokenCache = { token: null, expiresAt: 0 };

async function getFeishuAppToken() {
  const { FEISHU_APP_ID, FEISHU_APP_SECRET } = push_config;
  
  // 检查缓存的 token 是否有效
  if (feishuTokenCache.token && Date.now() < feishuTokenCache.expiresAt) {
    return feishuTokenCache.token;
  }
  
  return new Promise((resolve, reject) => {
    const options = {
      url: 'https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal',
      json: {
        app_id: FEISHU_APP_ID,
        app_secret: FEISHU_APP_SECRET
      },
      headers: {
        'Content-Type': 'application/json',
      },
      timeout,
    };
    
    $.post(options, (err, resp, data) => {
      try {
        if (err) {
          console.log('飞书获取token失败\n', err);
          reject(err);
        } else if (data.code === 0) {
          // 缓存 token，过期时间提前 5 分钟
          feishuTokenCache.token = data.tenant_access_token;
          feishuTokenCache.expiresAt = Date.now() + (data.expire - 300) * 1000;
          resolve(data.tenant_access_token);
        } else {
          console.log(`飞书获取token异常 ${data.msg}\n`);
          reject(new Error(data.msg));
        }
      } catch (e) {
        $.logErr(e, resp);
        reject(e);
      }
    });
  });
}

function feishuAppNotify(text, desp) {
  return new Promise(async (resolve) => {
    const { FEISHU_APP_ID, FEISHU_APP_SECRET, FEISHU_USER_ID } = push_config;
    
    if (!FEISHU_APP_ID || !FEISHU_APP_SECRET) {
      resolve();
      return;
    }
    
    try {
      // 获取 access_token
      const token = await getFeishuAppToken();
      
      // 构建消息内容
      const messageContent = `${text}\n\n${desp}`;
      
      let body;
      // 默认使用 union_id 类型（跨应用可用，飞书官方推荐）
      const receiveIdType = 'union_id';
      
      if (FEISHU_USER_ID) {
        // 发送给指定用户
        body = {
          receive_id: FEISHU_USER_ID,
          msg_type: 'text',
          content: JSON.stringify({ text: messageContent })
        };
      } else {
        // 没有指定用户，发送文本消息（需要先获取机器人所在群的用户列表等，这里简化处理）
        console.log('飞书应用消息：未配置用户ID，请配置 FEISHU_USER_ID');
        resolve();
        return;
      }
      
      const options = {
        url: `https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=${receiveIdType}`,
        json: body,
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        },
        timeout,
      };
      
      $.post(options, (err, resp, data) => {
        try {
          if (err) {
            console.log('飞书应用发送消息失败\n', err);
          } else if (data.code === 0) {
            console.log('飞书应用发送消息成功\n');
          } else if (data.code === 99991663) {
            // token 过期，重新获取后重试
            feishuTokenCache = { token: null, expiresAt: 0 };
            console.log('飞书应用token过期，重新获取...\n');
            feishuAppNotify(text, desp).then(resolve).catch(() => resolve());
          } else {
            console.log(`飞书应用发送消息异常 ${data.msg}\n`);
          }
        } catch (e) {
          $.logErr(e, resp);
        } finally {
          resolve(data);
        }
      });
    } catch (e) {
      console.log('飞书应用发送消息异常:', e.message);
      resolve();
    }
  });
}

function pushMeNotify(text, desp, params = {}) {
  return new Promise((resolve) => {
    const { PUSHME_KEY, PUSHME_URL } = push_config;
    if (PUSHME_KEY) {
      const options = {
        url: PUSHME_URL || 'https://push.i-i.me',
        json: { push_key: PUSHME_KEY, title: text, content: desp, ...params },
        headers: {
          'Content-Type': 'application/json',
        },
        timeout,
      };
      $.post(options, (err, resp, data) => {
        try {
          if (err) {
            console.log('PushMe 发送通知调用API失败😞\n', err);
          } else {
            if (data === 'success') {
              console.log('PushMe 发送通知消息成功🎉\n');
            } else {
              console.log(`PushMe 发送通知消息异常 ${data}\n`);
            }
          }
        } catch (e) {
          $.logErr(e, resp);
        } finally {
          resolve(data);
        }
      });
    } else {
      resolve();
    }
  });
}

function chronocatNotify(title, desp) {
  return new Promise((resolve) => {
    const { CHRONOCAT_TOKEN, CHRONOCAT_QQ, CHRONOCAT_URL } = push_config;
    if (!CHRONOCAT_TOKEN || !CHRONOCAT_QQ || !CHRONOCAT_URL) {
      resolve();
      return;
    }

    const user_ids = CHRONOCAT_QQ.match(/user_id=(\d+)/g)?.map(
      (match) => match.split('=')[1],
    );
    const group_ids = CHRONOCAT_QQ.match(/group_id=(\d+)/g)?.map(
      (match) => match.split('=')[1],
    );

    const url = `${CHRONOCAT_URL}/api/message/send`;
    const headers = {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${CHRONOCAT_TOKEN}`,
    };

    for (const [chat_type, ids] of [
      [1, user_ids],
      [2, group_ids],
    ]) {
      if (!ids) {
        continue;
      }
      for (const chat_id of ids) {
        const data = {
          peer: {
            chatType: chat_type,
            peerUin: chat_id,
          },
          elements: [
            {
              elementType: 1,
              textElement: {
                content: `${title}\n\n${desp}`,
              },
            },
          ],
        };
        const options = {
          url: url,
          json: data,
          headers,
          timeout,
        };
        $.post(options, (err, resp, data) => {
          try {
            if (err) {
              console.log('Chronocat 发送QQ通知消息失败😞\n', err);
            } else {
              if (chat_type === 1) {
                console.log(`Chronocat 个人消息 ${ids}推送成功🎉`);
              } else {
                console.log(`Chronocat 群消息 ${ids}推送成功🎉`);
              }
            }
          } catch (e) {
            $.logErr(e, resp);
          } finally {
            resolve(data);
          }
        });
      }
    }
  });
}

function qmsgNotify(text, desp) {
  return new Promise((resolve) => {
    const { QMSG_KEY, QMSG_TYPE } = push_config;
    if (QMSG_KEY && QMSG_TYPE) {
      const options = {
        url: `https://qmsg.zendee.cn/${QMSG_TYPE}/${QMSG_KEY}`,
        body: `msg=${text}\n\n${desp.replace('----', '-')}`,
        headers: {
          'Content-Type': 'application/x-www-form-urlencoded',
        },
        timeout,
      };
      $.post(options, (err, resp, data) => {
        try {
          if (err) {
            console.log('Qmsg 发送通知调用API失败😞\n', err);
          } else {
            if (data.code === 0) {
              console.log('Qmsg 发送通知消息成功🎉\n');
            } else {
              console.log(`Qmsg 发送通知消息异常 ${data}\n`);
            }
          }
        } catch (e) {
          $.logErr(e, resp);
        } finally {
          resolve(data);
        }
      });
    } else {
      resolve();
    }
  });
}

function webhookNotify(text, desp) {
  return new Promise((resolve) => {
    const {
      WEBHOOK_URL,
      WEBHOOK_BODY,
      WEBHOOK_HEADERS,
      WEBHOOK_CONTENT_TYPE,
      WEBHOOK_METHOD,
    } = push_config;
    if (
      !WEBHOOK_METHOD ||
      !WEBHOOK_URL ||
      (!WEBHOOK_URL.includes('$title') && !WEBHOOK_BODY.includes('$title'))
    ) {
      resolve();
      return;
    }

    const headers = parseHeaders(WEBHOOK_HEADERS);
    const body = parseBody(WEBHOOK_BODY, WEBHOOK_CONTENT_TYPE, (v) =>
      v
        ?.replaceAll('$title', text?.replaceAll('\n', '\\n'))
        ?.replaceAll('$content', desp?.replaceAll('\n', '\\n')),
    );
    const bodyParam = formatBodyFun(WEBHOOK_CONTENT_TYPE, body);
    const options = {
      method: WEBHOOK_METHOD,
      headers,
      allowGetBody: true,
      ...bodyParam,
      timeout,
      retry: 1,
    };

    const formatUrl = WEBHOOK_URL.replaceAll(
      '$title',
      encodeURIComponent(text),
    ).replaceAll('$content', encodeURIComponent(desp));
    httpClient.request(formatUrl, options).then(async (resp) => {
      const body = await resp.body.text();
      try {
        if (resp.statusCode !== 200) {
          console.log(`自定义发送通知消息失败😞 ${body}\n`);
        } else {
          console.log(`自定义发送通知消息成功🎉 ${body}\n`);
        }
      } catch (e) {
        $.logErr(e, resp);
      } finally {
        resolve(body);
      }
    });
  });
}

function ntfyNotify(text, desp) {
  function encodeRFC2047(text) {
    const encodedBase64 = Buffer.from(text).toString('base64');
    return `=?utf-8?B?${encodedBase64}?=`;
  }

  return new Promise((resolve) => {
    const {
      NTFY_URL,
      NTFY_TOPIC,
      NTFY_PRIORITY,
      NTFY_TOKEN,
      NTFY_USERNAME,
      NTFY_PASSWORD,
      NTFY_ACTIONS,
    } = push_config;
    if (NTFY_TOPIC) {
      const options = {
        url: `${NTFY_URL || 'https://ntfy.sh'}/${NTFY_TOPIC}`,
        body: `${desp}`,
        headers: {
          Title: `${encodeRFC2047(text)}`,
          Priority: NTFY_PRIORITY || '3',
          Icon: 'https://qn.whyour.cn/logo.png',
        },
        timeout,
      };
      if (NTFY_TOKEN) {
        options.headers['Authorization'] = `Bearer ${NTFY_TOKEN}`;
      } else if (NTFY_USERNAME && NTFY_PASSWORD) {
        options.headers['Authorization'] =
          `Basic ${Buffer.from(`${NTFY_USERNAME}:${NTFY_PASSWORD}`).toString('base64')}`;
      }
      if (NTFY_ACTIONS) {
        options.headers['Actions'] = encodeRFC2047(NTFY_ACTIONS);
      }

      $.post(options, (err, resp, data) => {
        try {
          if (err) {
            console.log('Ntfy 通知调用API失败😞\n', err);
          } else {
            if (data.id) {
              console.log('Ntfy 发送通知消息成功🎉\n');
            } else {
              console.log(`Ntfy 发送通知消息异常 ${JSON.stringify(data)}`);
            }
          }
        } catch (e) {
          $.logErr(e, resp);
        } finally {
          resolve(data);
        }
      });
    } else {
      resolve();
    }
  });
}

function wxPusherNotify(text, desp) {
  return new Promise((resolve) => {
    const { WXPUSHER_APP_TOKEN, WXPUSHER_TOPIC_IDS, WXPUSHER_UIDS } =
      push_config;
    if (WXPUSHER_APP_TOKEN) {
      // 处理topic_ids，将分号分隔的字符串转为数组
      const topicIds = WXPUSHER_TOPIC_IDS
        ? WXPUSHER_TOPIC_IDS.split(';')
            .map((id) => id.trim())
            .filter((id) => id)
            .map((id) => parseInt(id))
        : [];

      // 处理uids，将分号分隔的字符串转为数组
      const uids = WXPUSHER_UIDS
        ? WXPUSHER_UIDS.split(';')
            .map((uid) => uid.trim())
            .filter((uid) => uid)
        : [];

      // topic_ids uids 至少有一个
      if (!topicIds.length && !uids.length) {
        console.log(
          'wxpusher 服务的 WXPUSHER_TOPIC_IDS 和 WXPUSHER_UIDS 至少设置一个!!',
        );
        return resolve();
      }

      const body = {
        appToken: WXPUSHER_APP_TOKEN,
        content: `<h1>${text}</h1><br/><div style='white-space: pre-wrap;'>${desp}</div>`,
        summary: text,
        contentType: 2,
        topicIds: topicIds,
        uids: uids,
        verifyPayType: 0,
      };

      const options = {
        url: 'https://wxpusher.zjiecode.com/api/send/message',
        body: JSON.stringify(body),
        headers: {
          'Content-Type': 'application/json',
        },
        timeout,
      };

      $.post(options, (err, resp, data) => {
        try {
          if (err) {
            console.log('wxpusher发送通知消息失败！\n', err);
          } else {
            if (data.code === 1000) {
              console.log('wxpusher发送通知消息完成！');
            } else {
              console.log(`wxpusher发送通知消息异常：${data.msg}`);
            }
          }
        } catch (e) {
          $.logErr(e, resp);
        } finally {
          resolve(data);
        }
      });
    } else {
      resolve();
    }
  });
}

// QQ机器人相关常量
const QQ_API_BASE = 'https://api.sgroup.qq.com';
const QQ_TOKEN_URL = 'https://bots.qq.com/app/getAppAccessToken';

// QQ机器人Token缓存
let qqCachedToken = null;

/**
 * 获取QQ机器人AccessToken
 */
async function getQQAccessToken(appId, appSecret) {
  const now = Date.now();
  if (qqCachedToken && now < qqCachedToken.expiresAt - 5 * 60 * 1000 && qqCachedToken.appId === appId) {
    return qqCachedToken.token;
  }

  if (qqCachedToken && qqCachedToken.appId !== appId) {
    qqCachedToken = null;
  }

  const options = {
    url: QQ_TOKEN_URL,
    json: { appId: appId, clientSecret: appSecret },
    headers: { 'Content-Type': 'application/json' },
    timeout,
  };

  return new Promise((resolve, reject) => {
    $.post(options, (err, resp, data) => {
      if (err) {
        reject(err);
      } else {
        const token = data.access_token;
        if (!token) {
          reject(new Error(`获取Token失败`));
          return;
        }
        const expiresIn = parseInt(data.expires_in) || 7200;
        qqCachedToken = {
          token: token,
          expiresAt: now + expiresIn * 1000,
          appId: appId,
        };
        resolve(token);
      }
    });
  });
}

/**
 * 发送QQ消息（单聊或群聊）
 */
async function sendQQMessage(accessToken, target, content, isGroup, useMarkdown = false) {
  const path = isGroup 
    ? `/v2/groups/${target}/messages`
    : `/v2/users/${target}/messages`;
  
  const body = useMarkdown
    ? { markdown: { content: content }, msg_type: 2 }
    : { content: content, msg_type: 0 };

  const options = {
    url: `${QQ_API_BASE}${path}`,
    json: body,
    headers: {
      'Authorization': `QQBot ${accessToken}`,
      'Content-Type': 'application/json',
    },
    timeout,
  };

  return new Promise((resolve, reject) => {
    $.post(options, (err, resp, data) => {
      if (err) {
        reject(err);
      } else {
        resolve(data);
      }
    });
  });
}

/**
 * 获取QQ Gateway URL
 */
async function getQQGatewayUrl(accessToken) {
  const options = {
    url: `${QQ_API_BASE}/gateway`,
    headers: {
      'Authorization': `QQBot ${accessToken}`,
      'Content-Type': 'application/json',
    },
    timeout,
  };

  return new Promise((resolve, reject) => {
    $.get(options, (err, resp, data) => {
      if (err) {
        reject(err);
      } else {
        if (data && data.url) {
          resolve(data.url);
        } else {
          reject(new Error(`Gateway URL not found`));
        }
      }
    });
  });
}

// QQ Bot intents
// 参考: https://bot.q.qq.com/wiki/develop/api/gateway/intents.html
const QQ_INTENT_PUBLIC_GUILD_MESSAGES = 1 << 3;  // 公域消息事件
const QQ_INTENT_GUILD_MESSAGES = 1 << 9;          // 频道消息事件
const QQ_INTENT_GROUP_AND_C2C = 1 << 25;          // 群聊和私聊事件

// 组合所有消息相关的 intents
const QQ_INTENT_ALL_MESSAGES = QQ_INTENT_PUBLIC_GUILD_MESSAGES | QQ_INTENT_GUILD_MESSAGES | QQ_INTENT_GROUP_AND_C2C;

/**
 * 启动QQ openid监听器（需要安装 ws 库: npm install ws）
 */
async function startQQOpenidListener(appId, appSecret) {
  let WebSocket;
  try {
    WebSocket = require('ws');
  } catch (e) {
    console.log('QQ机器人: 缺少 ws 库，请运行: npm install ws');
    console.log(`QQ机器人: 错误详情: ${e.message || e}`);
    return;
  }

  let accessToken;
  let gatewayUrl;
  try {
    accessToken = await getQQAccessToken(appId, appSecret);
    gatewayUrl = await getQQGatewayUrl(accessToken);
  } catch (e) {
    console.log(`QQ机器人: 获取Gateway失败: ${e.message || e}`);
    return;
  }

  if (!gatewayUrl) {
    console.log('QQ机器人: Gateway URL 为空');
    return;
  }

  console.log('QQ机器人: 正在连接Gateway...');

  return new Promise((resolve) => {
    const ws = new WebSocket(gatewayUrl);
    let lastSeq = null;
    let identified = false;
    let heartbeatTimer = null;
    let timeoutTimer = null;

    const cleanup = () => {
      if (heartbeatTimer) clearTimeout(heartbeatTimer);
      if (timeoutTimer) clearTimeout(timeoutTimer);
      if (ws.readyState === WebSocket.OPEN) ws.close();
    };

    // 60秒超时
    timeoutTimer = setTimeout(() => {
      console.log('QQ机器人: 监听结束（60秒超时）');
      cleanup();
      resolve();
    }, 60000);

    ws.on('open', () => {
      console.log('QQ机器人: WebSocket已连接');
    });

    ws.on('message', (data) => {
      let payload;
      try {
        payload = JSON.parse(data);
      } catch (e) {
        return;
      }

      const op = payload.op;
      const d = payload.d;
      const s = payload.s;
      const t = payload.t;

      if (s !== undefined) lastSeq = s;

      if (op === 10) { // Hello
        const heartbeatInterval = d.heartbeat_interval || 30000;

        // Identify
        const identify = {
          op: 2,
          d: {
            token: `QQBot ${accessToken}`,
            intents: QQ_INTENT_GROUP_AND_C2C,
            shard: [0, 1],
          },
        };
        ws.send(JSON.stringify(identify));
        identified = true;

        // 启动心跳
        const sendHeartbeat = () => {
          if (ws.readyState === WebSocket.OPEN) {
            ws.send(JSON.stringify({ op: 1, d: lastSeq }));
            heartbeatTimer = setTimeout(sendHeartbeat, heartbeatInterval);
          }
        };
        heartbeatTimer = setTimeout(sendHeartbeat, heartbeatInterval);
      } else if (op === 0) { // Dispatch
        if (t === 'READY') {
          console.log('QQ机器人: 认证成功！等待消息...');
        } else if (t === 'C2C_MESSAGE_CREATE') {
          const author = d.author || {};
          const userOpenid = author.user_openid || '';
          const msgContent = (d.content || '').trim();

          console.log('\n============================================================');
          console.log('收到私聊消息');
          console.log('============================================================');
          console.log(`用户 openid: ${userOpenid}`);
          console.log(`消息内容: ${msgContent}`);
          console.log('------------------------------------------------------------');
          console.log('请将以下配置添加到环境变量：');
          console.log(`  export QQ_OPENID="${userOpenid}"`);
          console.log('============================================================\n');

          cleanup();
          resolve();
        } else if (t === 'GROUP_AT_MESSAGE_CREATE') {
          const groupOpenid = d.group_openid || '';
          const msgContent = (d.content || '').trim();

          console.log('\n============================================================');
          console.log('收到群聊@消息');
          console.log('============================================================');
          console.log(`群 openid: ${groupOpenid}`);
          console.log(`消息内容: ${msgContent}`);
          console.log('------------------------------------------------------------');
          console.log('请将以下配置添加到环境变量：');
          console.log(`  export QQ_GROUP_OPENID="${groupOpenid}"`);
          console.log('============================================================\n');

          cleanup();
          resolve();
        } else if (t === 'GROUP_MESSAGE_CREATE') {
          // 群消息（不需要@）
          const groupOpenid = d.group_openid || '';
          const author = d.author || {};
          const msgContent = (d.content || '').trim();

          console.log('\n============================================================');
          console.log('收到群消息（非@）');
          console.log('============================================================');
          console.log(`群 openid: ${groupOpenid}`);
          console.log(`发送者: ${author.user_openid || '未知'}`);
          console.log(`消息内容: ${msgContent}`);
          console.log('------------------------------------------------------------');
          console.log('如需使用此群，请将以下配置添加到环境变量：');
          console.log(`  export QQ_GROUP_OPENID="${groupOpenid}"`);
          console.log('============================================================\n');

          cleanup();
          resolve();
        } else if (t === 'DIRECT_MESSAGE_CREATE') {
          // 频道私信
          const author = d.author || {};
          const userOpenid = author.user_openid || '';
          const msgContent = (d.content || '').trim();

          console.log('\n============================================================');
          console.log('收到频道私信');
          console.log('============================================================');
          console.log(`用户 openid: ${userOpenid}`);
          console.log(`消息内容: ${msgContent}`);
          console.log('------------------------------------------------------------');
          console.log('请将以下配置添加到环境变量：');
          console.log(`  export QQ_OPENID="${userOpenid}"`);
          console.log('============================================================\n');

          cleanup();
          resolve();
        } else if (t === 'AT_MESSAGE_CREATE') {
          // 频道@消息
          const channelId = d.channel_id || '';
          const author = d.author || {};
          const msgContent = (d.content || '').trim();

          console.log('\n============================================================');
          console.log('收到频道@消息');
          console.log('============================================================');
          console.log(`频道 ID: ${channelId}`);
          console.log(`用户 openid: ${author.user_openid || '未知'}`);
          console.log(`消息内容: ${msgContent}`);
          console.log('============================================================\n');
        }
      } else if (op === 11) { // Heartbeat ACK
        // 心跳响应，忽略
      } else if (op === 9) { // Invalid Session
        console.log(`QQ机器人: 会话无效: ${JSON.stringify(d)}`);
        cleanup();
        resolve();
      }
    });

    ws.on('error', (err) => {
      console.log(`QQ机器人: WebSocket错误: ${err}`);
      cleanup();
      resolve();
    });

    ws.on('close', () => {
      console.log('QQ机器人: 连接已关闭');
      cleanup();
      resolve();
    });
  });
}

/**
 * QQ机器人通知
 */
function qqBotNotify(text, desp) {
  return new Promise(async (resolve) => {
    const { QQ_APP_ID, QQ_APP_SECRET, QQ_OPENID, QQ_GROUP_OPENID } = push_config;
    
    if (!QQ_APP_ID || !QQ_APP_SECRET) {
      resolve();
      return;
    }

    console.log('QQ机器人 服务启动');

    // 确定发送目标
    const targets = [];
    if (QQ_GROUP_OPENID) {
      targets.push({ id: QQ_GROUP_OPENID, isGroup: true });
    }
    if (QQ_OPENID) {
      targets.push({ id: QQ_OPENID, isGroup: false });
    }

    if (targets.length === 0) {
      // 未配置接收者时，启动监听模式获取openid
      console.log('QQ机器人: 未配置接收者，启动监听模式获取openid...');
      await startQQOpenidListener(QQ_APP_ID, QQ_APP_SECRET);
      resolve();
      return;
    }

    // 格式化消息内容（Markdown格式）
    const content = `**${text}**\n\n${desp}`;

    try {
      const accessToken = await getQQAccessToken(QQ_APP_ID, QQ_APP_SECRET);
      let successCount = 0;

      for (const target of targets) {
        try {
          await sendQQMessage(accessToken, target.id, content, target.isGroup, true);
          successCount++;
        } catch (err) {
          const errMsg = err.message || String(err);
          // 如果Markdown不可用，回退到纯文本
          if (errMsg.includes('markdown') || errMsg.includes('11244') || errMsg.includes('权限')) {
            try {
              const plainContent = `【${text}】\n\n${desp}`;
              await sendQQMessage(accessToken, target.id, plainContent, target.isGroup, false);
              successCount++;
            } catch (fallbackErr) {
              console.log(`QQ机器人: 发送失败 (${target.id}): ${fallbackErr}`);
            }
          } else {
            console.log(`QQ机器人: 发送失败 (${target.id}): ${err}`);
          }
        }
      }

      if (successCount > 0) {
        console.log('QQ机器人 推送成功！');
      }
    } catch (err) {
      console.log('QQ机器人 推送失败！', err);
    }

    resolve();
  });
}

function parseString(input, valueFormatFn) {
  const regex = /(\w+):\s*((?:(?!\n\w+:).)*)/g;
  const matches = {};

  let match;
  while ((match = regex.exec(input)) !== null) {
    const [, key, value] = match;
    const _key = key.trim();
    if (!_key || matches[_key]) {
      continue;
    }

    let _value = value.trim();

    try {
      _value = valueFormatFn ? valueFormatFn(_value) : _value;
      const jsonValue = JSON.parse(_value);
      matches[_key] = jsonValue;
    } catch (error) {
      matches[_key] = _value;
    }
  }

  return matches;
}

function parseHeaders(headers) {
  if (!headers) return {};

  const parsed = {};
  let key;
  let val;
  let i;

  headers &&
    headers.split('\n').forEach(function parser(line) {
      i = line.indexOf(':');
      key = line.substring(0, i).trim().toLowerCase();
      val = line.substring(i + 1).trim();

      if (!key) {
        return;
      }

      parsed[key] = parsed[key] ? parsed[key] + ', ' + val : val;
    });

  return parsed;
}

function parseBody(body, contentType, valueFormatFn) {
  if (contentType === 'text/plain' || !body) {
    return valueFormatFn && body ? valueFormatFn(body) : body;
  }

  const parsed = parseString(body, valueFormatFn);

  switch (contentType) {
    case 'multipart/form-data':
      return Object.keys(parsed).reduce((p, c) => {
        p.append(c, parsed[c]);
        return p;
      }, new FormData());
    case 'application/x-www-form-urlencoded':
      return Object.keys(parsed).reduce((p, c) => {
        return p ? `${p}&${c}=${parsed[c]}` : `${c}=${parsed[c]}`;
      });
  }

  return parsed;
}

function formatBodyFun(contentType, body) {
  if (!body) return {};
  switch (contentType) {
    case 'application/json':
      return { json: body };
    case 'multipart/form-data':
      return { form: body };
    case 'application/x-www-form-urlencoded':
    case 'text/plain':
      return { body };
  }
  return {};
}

// ==================== 企业微信智能机器人（长连接模式）====================
let wechatBotClient = null;

function getWechatBotClient() {
  if (wechatBotClient) {
    return wechatBotClient;
  }

  const { WECHAT_BOT_ID, WECHAT_BOT_SECRET, WECHAT_BOT_WS_URL } = push_config;
  if (!WECHAT_BOT_ID || !WECHAT_BOT_SECRET) {
    return null;
  }

  const WebSocket = require('ws');
  const crypto = require('crypto');

  class WechatBotClient {
    constructor() {
      this.botId = WECHAT_BOT_ID;
      this.botSecret = WECHAT_BOT_SECRET;
      this.wsUrl = WECHAT_BOT_WS_URL;
      this.ws = null;
      this.authenticated = false;
      this.pendingAcks = new Map();
      this.knownTargets = new Set();
      this.heartbeatInterval = null;
      this.reconnectTimer = null;
      this.connect();
    }

    buildReqId(prefix) {
      return `${prefix}_${crypto.randomUUID().replace(/-/g, '')}`;
    }

    connect() {
      if (this.ws) {
        return;
      }

      try {
        this.ws = new WebSocket(this.wsUrl);
        
        this.ws.on('open', () => {
          console.log('企业微信智能机器人连接成功，开始订阅...');
          this.sendRaw({
            cmd: 'aibot_subscribe',
            headers: { req_id: this.buildReqId('aibot_subscribe') },
            body: {
              bot_id: this.botId,
              secret: this.botSecret,
            },
          });
        });

        this.ws.on('message', (data) => {
          try {
            const payload = JSON.parse(data.toString());
            this.handleMessage(payload);
          } catch (e) {
            console.error('解析企业微信智能机器人消息失败:', e);
          }
        });

        this.ws.on('error', (err) => {
          console.error('企业微信智能机器人 WebSocket 错误:', err.message);
          this.authenticated = false;
        });

        this.ws.on('close', () => {
          console.log('企业微信智能机器人连接关闭');
          this.authenticated = false;
          this.ws = null;
          this.scheduleReconnect();
        });

        // 启动心跳
        this.startHeartbeat();
      } catch (e) {
        console.error('企业微信智能机器人连接失败:', e.message);
        this.scheduleReconnect();
      }
    }

    scheduleReconnect() {
      if (this.reconnectTimer) {
        return;
      }
      this.reconnectTimer = setTimeout(() => {
        this.reconnectTimer = null;
        this.connect();
      }, 10000);
    }

    startHeartbeat() {
      if (this.heartbeatInterval) {
        clearInterval(this.heartbeatInterval);
      }
      this.heartbeatInterval = setInterval(() => {
        if (this.authenticated && this.ws && this.ws.readyState === WebSocket.OPEN) {
          this.sendRaw({
            cmd: 'ping',
            headers: { req_id: this.buildReqId('ping') },
          });
        }
      }, 30000);
    }

    sendRaw(payload) {
      if (this.ws && this.ws.readyState === WebSocket.OPEN) {
        this.ws.send(JSON.stringify(payload));
      }
    }

    handleMessage(payload) {
      const reqId = (payload.headers || {}).req_id;
      
      // 处理 ACK
      if (reqId && this.pendingAcks.has(reqId)) {
        const pending = this.pendingAcks.get(reqId);
        this.pendingAcks.delete(reqId);
        pending.resolve(payload);
      }

      // 处理订阅响应
      if (String(reqId).startsWith('aibot_subscribe')) {
        if (payload.errcode === 0) {
          this.authenticated = true;
          console.log('企业微信智能机器人订阅成功');
        } else {
          console.error('企业微信智能机器人订阅失败:', payload.errmsg);
        }
        return;
      }

      // 处理消息回调
      const cmd = payload.cmd;
      if (cmd === 'aibot_msg_callback') {
        this.handleCallbackMessage(payload);
      }
    }

    handleCallbackMessage(payload) {
      const body = payload.body || {};
      const sender = (body.from || {}).userid || '';
      if (!sender) return;

      // 记录已互动用户
      if (!this.knownTargets.has(sender)) {
        this.knownTargets.add(sender);
        console.log(`企业微信智能机器人记录用户: ${sender}`);
      }
    }

    async sendWithAck(payload, timeout = 10000) {
      return new Promise((resolve) => {
        const reqId = (payload.headers || {}).req_id;
        if (!reqId) {
          resolve({ errcode: -1, errmsg: '缺少 req_id' });
          return;
        }

        const timer = setTimeout(() => {
          this.pendingAcks.delete(reqId);
          resolve({ errcode: -1, errmsg: '发送超时' });
        }, timeout);

        this.pendingAcks.set(reqId, {
          resolve: (data) => {
            clearTimeout(timer);
            resolve(data);
          },
        });

        this.sendRaw(payload);
      });
    }

    normalizeTarget(chatId) {
      if (!chatId) return { target: null, chatType: 1 };
      const lowered = chatId.toLowerCase();
      if (lowered.startsWith('group:')) {
        return { target: chatId.substring(6).trim(), chatType: 2 };
      }
      if (lowered.startsWith('user:')) {
        return { target: chatId.substring(5).trim(), chatType: 1 };
      }
      return { target: chatId.trim(), chatType: 1 };
    }

    getTargets(chatId) {
      const { target, chatType } = this.normalizeTarget(chatId);
      if (target) {
        return [{ target, chatType }];
      }
      // 如果没有指定目标，发送给所有已互动用户
      return Array.from(this.knownTargets).map((userid) => ({
        target: userid,
        chatType: 1,
      }));
    }

    splitContent(content, maxBytes = 4000) {
      if (!content) return [];
      const chunks = [];
      let current = '';
      for (const line of content.split('\n')) {
        const newContent = current + (current ? '\n' : '') + line;
        if (Buffer.byteLength(newContent, 'utf8') > maxBytes) {
          if (current) chunks.push(current);
          current = line;
        } else {
          current = newContent;
        }
      }
      if (current) chunks.push(current);
      return chunks;
    }

    async sendMarkdown(content, chatId) {
      if (!this.authenticated) {
        console.log('企业微信智能机器人未认证，尝试连接...');
        return false;
      }

      const targets = this.getTargets(chatId);
      if (targets.length === 0) {
        console.log('企业微信智能机器人没有可发送的目标');
        return false;
      }

      let success = false;
      for (const { target, chatType } of targets) {
        for (const chunk of this.splitContent(content)) {
          const payload = {
            cmd: 'aibot_send_msg',
            headers: { req_id: this.buildReqId('aibot_send_msg') },
            body: {
              chatid: target,
              chat_type: chatType,
              msgtype: 'markdown',
              markdown: { content: chunk },
            },
          };
          const ack = await this.sendWithAck(payload);
          if (ack.errcode === 0) {
            success = true;
          } else {
            console.error(`企业微信智能机器人发送失败: ${ack.errmsg}`);
          }
        }
      }
      return success;
    }

    async sendMsg(title, text, chatId) {
      const content = `**${title}**\n\n${text || ''}`.trim();
      return this.sendMarkdown(content, chatId);
    }
  }

  wechatBotClient = new WechatBotClient();
  return wechatBotClient;
}

async function wechatBotNotify(text, desp) {
  const client = getWechatBotClient();
  if (!client) {
    return;
  }
  
  const { WECHAT_BOT_CHAT_ID } = push_config;
  
  // 等待认证完成（最多等待10秒）
  const maxWaitTime = 10000;
  const checkInterval = 500;
  let waitedTime = 0;
  
  while (!client.authenticated && waitedTime < maxWaitTime) {
    console.log(`企业微信智能机器人等待认证... (${waitedTime/1000}s)`);
    await new Promise(resolve => setTimeout(resolve, checkInterval));
    waitedTime += checkInterval;
  }
  
  if (!client.authenticated) {
    console.error('企业微信智能机器人认证超时，发送失败');
    throw new Error('企业微信智能机器人认证超时');
  }
  
  try {
    await client.sendMsg(text, desp, WECHAT_BOT_CHAT_ID);
  } catch (e) {
    console.error('企业微信智能机器人发送失败:', e.message);
    throw e;
  }
}

/**
 * sendNotify 推送通知功能
 * @param text 通知头
 * @param desp 通知体
 * @param params 某些推送通知方式点击弹窗可跳转, 例：{ url: 'https://abc.com' }
 * @returns {Promise<unknown>}
 */
async function sendNotify(text, desp, params = {}) {
  // 根据标题跳过一些消息推送，环境变量：SKIP_PUSH_TITLE 用回车分隔
  let skipTitle = process.env.SKIP_PUSH_TITLE;
  if (skipTitle) {
    if (skipTitle.split('\n').includes(text)) {
      console.info(text + '在 SKIP_PUSH_TITLE 环境变量内，跳过推送');
      return;
    }
  }

  await Promise.all([
    serverNotify(text, desp), // 微信server酱
    pushPlusNotify(text, desp), // pushplus
    wePlusBotNotify(text, desp), // 微加机器人
    barkNotify(text, desp, params), // iOS Bark APP
    tgBotNotify(text, desp), // telegram 机器人
    ddBotNotify(text, desp), // 钉钉机器人
    qywxBotNotify(text, desp), // 企业微信机器人
    qywxamNotify(text, desp), // 企业微信应用消息推送
    wechatBotNotify(text, desp), // 企业微信智能机器人（长连接）
    iGotNotify(text, desp, params), // iGot
    gotifyNotify(text, desp), // gotify
    chatNotify(text, desp), // synolog chat
    pushDeerNotify(text, desp), // PushDeer
    aibotkNotify(text, desp), // 智能微秘书
    fsBotNotify(text, desp), // 飞书自定义机器人
    feishuAppNotify(text, desp), // 飞书企业自建应用
    pushMeNotify(text, desp, params), // PushMe
    chronocatNotify(text, desp), // Chronocat
    webhookNotify(text, desp), // 自定义通知
    qmsgNotify(text, desp), // qmsg酱
    ntfyNotify(text, desp), // Ntfy
    wxPusherNotify(text, desp), // wxpusher
    qqBotNotify(text, desp), // QQ机器人
  ]);
}

module.exports = {
  sendNotify,
};
