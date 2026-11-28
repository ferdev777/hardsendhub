import { Container } from "@cloudflare/containers";

export interface Env {
	HARDSEND_CONTAINER: any;
	EMAIL: any;
	DB: any;
	INVOICES_BUCKET: any;
	PORT?: string;
	WORKER_COUNT?: string;
	CF_ACCOUNT_ID?: string;
	CF_API_TOKEN?: string;
	CF_FROM_ADDRESS?: string;
	CF_RATE_LIMIT?: string;
	ADMIN_USERNAME?: string;
	ADMIN_PASSWORD?: string;
	JWT_SECRET?: string;
	CF_WORKER_URL?: string;
}

export class HardSendContainer extends Container<Env> {
	defaultPort = 8080;
	sleepAfter = "600s"; // Sleep after 10 minutes of inactivity (Scale-to-Zero)
	enableInternet = true;

	constructor(ctx: any, env: Env) {
		super(ctx, env);
		this.envVars = {
			PORT: env.PORT || "8080",
			WORKER_COUNT: env.WORKER_COUNT || "2",
			CF_ACCOUNT_ID: env.CF_ACCOUNT_ID || "974a2eb57b6467c57ede7c991e7b6f81",
			CF_API_TOKEN: env.CF_API_TOKEN || "",
			CF_FROM_ADDRESS: env.CF_FROM_ADDRESS || "facturas@facturasvideodigital.com",
			CF_RATE_LIMIT: env.CF_RATE_LIMIT || "15",
			ADMIN_USERNAME: env.ADMIN_USERNAME || "hardsendvideodigital",
			ADMIN_PASSWORD: env.ADMIN_PASSWORD || "",
			JWT_SECRET: env.JWT_SECRET || "",
			CF_WORKER_URL: env.CF_WORKER_URL || "https://app.facturasvideodigital.com",
		};
	}

	override onStart() {
		console.log("[HardSendContainer] Container started with injected envVars.");
	}

	override onStop() {
		console.log("[HardSendContainer] Container stopped (idle).");
	}
}

export default {
	async fetch(request: Request, env: Env, ctx: any): Promise<Response> {
		try {
			const url = new URL(request.url);

			// Endpoint: Real-time Delivery & Bounce Logs from D1 (with server-side search, filter & pagination)
			if (url.pathname === "/api/logs" && request.method === "GET") {
				if (env.DB) {
					try {
						const rawLimit = parseInt(url.searchParams.get("limit") || "100", 10);
						const limit = Number.isFinite(rawLimit) ? Math.min(Math.max(rawLimit, 1), 200) : 100;
						const rawOffset = parseInt(url.searchParams.get("offset") || "0", 10);
						const offset = Number.isFinite(rawOffset) && rawOffset >= 0 ? rawOffset : 0;
						const q = (url.searchParams.get("q") || "").trim();
						const filterType = (url.searchParams.get("type") || "ALL").toUpperCase();
						const paginated = url.searchParams.get("paginated") === "1";

						const whereClauses: string[] = [];
						const bindParams: any[] = [];

						if (filterType === "DELIVERED") {
							whereClauses.push("event_type IN ('delivered', 'accepted')");
						} else if (filterType === "BOUNCED") {
							whereClauses.push("event_type IN ('bounced', 'deferred')");
						} else if (filterType === "COMPLAINED") {
							whereClauses.push("event_type = 'complained'");
						} else if (filterType === "FAILED") {
							whereClauses.push("event_type IN ('failed', 'rejected')");
						}

						if (q.length > 0) {
							const like = `%${q}%`;
							whereClauses.push(
								"(LOWER(COALESCE(recipient, '')) LIKE LOWER(?) OR LOWER(COALESCE(invoice_id, '')) LIKE LOWER(?) OR LOWER(COALESCE(message_id, '')) LIKE LOWER(?) OR LOWER(COALESCE(bounce_reason, '')) LIKE LOWER(?))"
							);
							bindParams.push(like, like, like, like);
						}

						const whereSQL = whereClauses.length > 0 ? `WHERE ${whereClauses.join(" AND ")}` : "";

						const { results } = await env.DB.prepare(
							`SELECT * FROM delivery_events ${whereSQL} ORDER BY created_at DESC LIMIT ? OFFSET ?`
						).bind(...bindParams, limit, offset).all();

						if (paginated) {
							const countRow: any = await env.DB.prepare(
								`SELECT COUNT(*) as total FROM delivery_events ${whereSQL}`
							).bind(...bindParams).first();

							return new Response(JSON.stringify({
								items: results || [],
								total: countRow?.total || 0,
								limit,
								offset,
							}), {
								headers: { "Content-Type": "application/json" },
							});
						}

						return new Response(JSON.stringify(results || []), {
							headers: { "Content-Type": "application/json" },
						});
					} catch (e: any) {
						console.error("[D1 Logs Error]", e);
					}
				}
			}

			// Endpoint: Suppressions and Blacklist from Cloudflare API & D1
			if (url.pathname === "/api/suppressions" && request.method === "GET") {
				if (env.CF_ACCOUNT_ID && env.CF_API_TOKEN) {
					try {
						const cfRes = await fetch(
							`https://api.cloudflare.com/client/v4/accounts/${env.CF_ACCOUNT_ID}/email/sending/suppressions`,
							{
								headers: { Authorization: `Bearer ${env.CF_API_TOKEN}` },
							}
						);
						if (cfRes.ok) {
							const cfData: any = await cfRes.json();
							const list = cfData.result || [];
							const mapped = list.map((item: any) => ({
								email: item.email,
								reason: item.reason === "soft_bounce" ? "Rebote Suave" : (item.reason === "hard_bounce" ? "Rebote Duro" : (item.reason || "Supresión")),
								source: "cloudflare",
								original_invoice: "",
								created_at: item.created_at || new Date().toISOString(),
								expires_at: item.expires_at || null,
							}));
							return new Response(JSON.stringify(mapped), {
								headers: { "Content-Type": "application/json" },
							});
						}
					} catch (e: any) {
						console.error("[Suppressions Fetch Error]", e);
					}
				}
				if (env.DB) {
					const { results } = await env.DB.prepare(
						"SELECT * FROM suppressions ORDER BY created_at DESC LIMIT 100"
					).all();
					return new Response(JSON.stringify(results || []), {
						headers: { "Content-Type": "application/json" },
					});
				}
			}

			// Endpoint: R2 PDF Storage - Store PDF
			if (url.pathname.startsWith("/api/storage/invoices/") && request.method === "PUT") {
				if (env.INVOICES_BUCKET) {
					const parts = url.pathname.replace("/api/storage/invoices/", "").split("/");
					if (parts.length >= 2) {
						const campaignId = parts[0];
						const invoiceNumber = parts[1];
						const key = `invoices/${campaignId}/${invoiceNumber}.pdf`;
						await env.INVOICES_BUCKET.put(key, request.body, {
							httpMetadata: { contentType: "application/pdf" },
						});
						return new Response(JSON.stringify({ success: true, key }), {
							headers: { "Content-Type": "application/json" },
						});
					}
				}
				return new Response(JSON.stringify({ error: "Storage not configured or invalid key" }), { status: 400 });
			}

			// Endpoint: R2 PDF Storage - Retrieve PDF
			if (url.pathname.startsWith("/api/storage/invoices/") && request.method === "GET") {
				if (env.INVOICES_BUCKET) {
					const parts = url.pathname.replace("/api/storage/invoices/", "").split("/");
					if (parts.length >= 2) {
						const campaignId = parts[0];
						const invoiceNumber = parts[1];
						const key = `invoices/${campaignId}/${invoiceNumber}.pdf`;
						const obj = await env.INVOICES_BUCKET.get(key);
						if (!obj) {
							return new Response("Invoice PDF not found in R2", { status: 404 });
						}
						return new Response(obj.body, {
							headers: {
								"Content-Type": "application/pdf",
								"Content-Length": obj.size.toString(),
								"Cache-Control": "public, max-age=31536000, immutable",
							},
						});
					}
				}
				return new Response("Storage not configured", { status: 404 });
			}

			// Endpoint: D1 Monthly Campaign Status & Daily Limits
			if (url.pathname === "/api/d1/campaigns/active" && request.method === "GET") {
				if (env.DB) {
					try {
						const campaign: any = await env.DB.prepare(
							"SELECT * FROM campaigns WHERE status = 'ACTIVE' ORDER BY created_at DESC LIMIT 1"
						).first();

						if (!campaign) {
							return new Response(JSON.stringify({ active: false }), {
								headers: { "Content-Type": "application/json" },
							});
						}

						const counts: any = await env.DB.prepare(`
							SELECT 
								COUNT(*) as total,
								SUM(CASE WHEN status = 'QUEUED' THEN 1 ELSE 0 END) as queued,
								SUM(CASE WHEN status IN ('ACCEPTED', 'DELIVERED', 'SENDING') THEN 1 ELSE 0 END) as accepted,
								SUM(CASE WHEN status = 'DELIVERED' THEN 1 ELSE 0 END) as delivered,
								SUM(CASE WHEN status LIKE '%BOUNCED%' THEN 1 ELSE 0 END) as bounced,
								SUM(CASE WHEN status = 'SUPPRESSED' THEN 1 ELSE 0 END) as suppressed
							FROM invoices WHERE campaign_id = ?
						`).bind(campaign.id).first();

						const todayStr = new Date().toISOString().slice(0, 10);
						const usage: any = await env.DB.prepare(
							"SELECT sent_count, quota_limit FROM daily_usage WHERE date = ?"
						).bind(todayStr).first();

						const sentToday = usage?.sent_count || 0;
						const quotaLimit = campaign.daily_limit || usage?.quota_limit || 200;

						return new Response(JSON.stringify({
							active: true,
							campaign: {
								...campaign,
								total_invoices: counts?.total || campaign.total_invoices,
								queued_count: counts?.queued || 0,
								accepted_count: counts?.accepted || 0,
								delivered_count: Math.max(counts?.delivered || 0, campaign.delivered_count || 0),
								bounced_count: Math.max(counts?.bounced || 0, campaign.bounced_count || 0),
								suppressed_count: counts?.suppressed || 0,
							},
							today: {
								date: todayStr,
								sent_count: sentToday,
								daily_limit: quotaLimit,
								remaining_quota: Math.max(0, quotaLimit - sentToday),
							},
						}), {
							headers: { "Content-Type": "application/json" },
						});
					} catch (e: any) {
						console.error("[D1 Campaign Error]", e);
						return new Response(JSON.stringify({ error: e.message }), { status: 500, headers: { "Content-Type": "application/json" } });
					}
				}
			}

			// Endpoint: Create or Activate Monthly Campaign
			if (url.pathname === "/api/d1/campaigns/create" && request.method === "POST") {
				if (env.DB) {
					const body: any = await request.json();
					const id = body.id || new Date().toISOString().slice(0, 7);
					const name = body.name || `Facturación ${body.period || id}`;
					const period = body.period || id;
					const total = body.total_invoices || 0;
					const dailyLimit = body.daily_limit || 200;
					const archive = body.archive_filename || "PDF.rar";

					await env.DB.prepare("UPDATE campaigns SET status = 'COMPLETED' WHERE status = 'ACTIVE'").run();

					await env.DB.prepare(`
						INSERT INTO campaigns (id, period, name, archive_filename, total_invoices, daily_limit, status)
						VALUES (?, ?, ?, ?, ?, ?, 'ACTIVE')
						ON CONFLICT(id) DO UPDATE SET
							total_invoices = excluded.total_invoices,
							daily_limit = excluded.daily_limit,
							status = 'ACTIVE',
							updated_at = CURRENT_TIMESTAMP
					`).bind(id, period, name, archive, total, dailyLimit).run();

					return new Response(JSON.stringify({ success: true, campaign_id: id }), {
						headers: { "Content-Type": "application/json" },
					});
				}
			}

			// Endpoint: Bulk Ingest Invoices into D1
			if (url.pathname === "/api/d1/invoices/batch" && request.method === "POST") {
				if (env.DB) {
					const { campaign_id, invoices }: any = await request.json();
					if (invoices && Array.isArray(invoices) && invoices.length > 0) {
						const stmts = invoices.map((inv: any) => {
							const id = `${campaign_id}_${inv.invoice_number}`;
							return env.DB.prepare(`
								INSERT OR IGNORE INTO invoices 
								(id, campaign_id, invoice_number, client_code, client_name, recipient_email, due_date, pdf_r2_key, pdf_size, priority_wave, status)
								VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'QUEUED')
							`).bind(
								id,
								campaign_id,
								inv.invoice_number,
								inv.client_code || null,
								inv.client_name || null,
								inv.recipient_email,
								inv.due_date || null,
								inv.pdf_r2_key || `invoices/${campaign_id}/${inv.invoice_number}.pdf`,
								inv.pdf_size || 0,
								inv.priority_wave || 1
							);
						});

						for (let i = 0; i < stmts.length; i += 50) {
							const chunk = stmts.slice(i, i + 50);
							await env.DB.batch(chunk);
						}

						return new Response(JSON.stringify({ success: true, count: invoices.length }), {
							headers: { "Content-Type": "application/json" },
						});
					}
				}
			}

			// Endpoint: Query next queued invoices from D1
			if (url.pathname === "/api/d1/invoices/next-batch" && request.method === "GET") {
				if (env.DB) {
					const campaignId = url.searchParams.get("campaign_id");
					const limit = parseInt(url.searchParams.get("limit") || "100");
					const { results } = await env.DB.prepare(`
						SELECT * FROM invoices 
						WHERE campaign_id = ? AND status = 'QUEUED'
						ORDER BY priority_wave ASC, id ASC
						LIMIT ?
					`).bind(campaignId, limit).all();
					return new Response(JSON.stringify(results || []), {
						headers: { "Content-Type": "application/json" },
					});
				}
			}

			// Endpoint: Mark batch as sent/accepted in D1 and update daily quota
			if (url.pathname === "/api/d1/invoices/mark-batch-sent" && request.method === "POST") {
				if (env.DB) {
					const { campaign_id, sent_ids }: any = await request.json();
					if (sent_ids && Array.isArray(sent_ids) && sent_ids.length > 0) {
						const todayStr = new Date().toISOString().slice(0, 10);
						const stmts = sent_ids.map((id: string) => {
							return env.DB.prepare(`
								UPDATE invoices 
								SET status = 'ACCEPTED', sent_at = CURRENT_TIMESTAMP, attempts = attempts + 1
								WHERE id = ?
							`).bind(id);
						});

						stmts.push(
							env.DB.prepare(`
								INSERT INTO daily_usage (date, sent_count, quota_limit, updated_at)
								VALUES (?, ?, COALESCE((SELECT daily_limit FROM campaigns WHERE id = ?), 200), CURRENT_TIMESTAMP)
								ON CONFLICT(date) DO UPDATE SET
									sent_count = sent_count + ?,
									quota_limit = COALESCE((SELECT daily_limit FROM campaigns WHERE id = ?), quota_limit),
									updated_at = CURRENT_TIMESTAMP
							`).bind(todayStr, sent_ids.length, campaign_id, sent_ids.length, campaign_id)
						);

						stmts.push(
							env.DB.prepare(`
								UPDATE campaigns 
								SET accepted_count = accepted_count + ?, updated_at = CURRENT_TIMESTAMP
								WHERE id = ?
							`).bind(sent_ids.length, campaign_id)
						);

						for (let i = 0; i < stmts.length; i += 50) {
							const chunk = stmts.slice(i, i + 50);
							await env.DB.batch(chunk);
						}

						return new Response(JSON.stringify({ success: true, updated: sent_ids.length }), {
							headers: { "Content-Type": "application/json" },
						});
					}
				}
			}

			const id = env.HARDSEND_CONTAINER.idFromName("default");
			const stub = env.HARDSEND_CONTAINER.get(id);

			// For chunk uploads or body-heavy endpoints, buffer the stream into an ArrayBuffer first
			// so the incoming stream is fully drained before the container responds.
			// This prevents Cloudflare's "TypeError: Can't read from request stream after response has been sent".
			if (request.method === "POST" && request.body && (url.pathname.includes("/api/upload") || url.pathname.includes("/chunk"))) {
				const bodyBuffer = await request.arrayBuffer();
				const headers = new Headers(request.headers);
				headers.set("Content-Length", bodyBuffer.byteLength.toString());

				const bufferedRequest = new Request(request.url, {
					method: request.method,
					headers,
					body: bodyBuffer,
				});
				return await stub.fetch(bufferedRequest);
			}

			return await stub.fetch(request);
		} catch (err: any) {
			console.error("[Container Proxy Error]", err?.message, err?.stack, JSON.stringify(err?.cause || {}));
			return new Response(`[Container Proxy Error]: ${err?.message || err}\n${err?.stack || ""}`, {
				status: 502,
				headers: { "Content-Type": "text/plain" },
			});
		}
	},

	// Cloudflare Queues Event Consumer (Receives real-time Email Sending lifecycle events)
	async queue(batch: any, env: Env) {
		for (const message of batch.messages) {
			try {
				const event = message.body;
				const eventType = event.type;
				const payload = event.payload;

				if (payload && env.DB) {
					const id = payload.eventId || crypto.randomUUID();
					const messageId = payload.messageId;
					const recipient = payload.recipient;
					const bounceType = payload.bounce?.type || null;
					const bounceReason = payload.bounce?.reason || null;
					const smtpCode = payload.delivery?.smtpStatusCode || null;
					const smtpResponse = payload.delivery?.smtpResponse || null;
					const status = payload.delivery?.status || eventType;

					await env.DB.prepare(`
						INSERT OR IGNORE INTO delivery_events 
						(id, event_type, event_id, message_id, recipient, smtp_code, smtp_response, bounce_type, bounce_reason)
						VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
					`).bind(
						id,
						status,
						payload.eventId || null,
						messageId || null,
						recipient,
						smtpCode,
						smtpResponse,
						bounceType,
						bounceReason
					).run();

					if (bounceType === "hard" || status === "bounced" || status === "complained") {
						await env.DB.prepare(`
							INSERT OR REPLACE INTO suppressions (email, reason, source, created_at)
							VALUES (?, ?, 'cloudflare', datetime('now'))
						`).bind(recipient, bounceReason || "Hard bounce detectado por Cloudflare").run();
					}

					// Update invoice and campaign status in D1 if messageId matches
					if (messageId) {
						if (status === "delivered" || eventType === "message.delivered") {
							await env.DB.prepare(`
								UPDATE invoices SET status = 'DELIVERED', delivered_at = CURRENT_TIMESTAMP WHERE message_id = ?
							`).bind(messageId).run();
							await env.DB.prepare(`
								UPDATE campaigns SET delivered_count = delivered_count + 1 WHERE status = 'ACTIVE'
							`).run();
						} else if (bounceType === "hard" || status === "bounced") {
							await env.DB.prepare(`
								UPDATE invoices SET status = 'BOUNCED_HARD', bounced_at = CURRENT_TIMESTAMP, error_reason = ? WHERE message_id = ?
							`).bind(bounceReason || "Hard Bounce", messageId).run();
							await env.DB.prepare(`
								UPDATE campaigns SET bounced_count = bounced_count + 1 WHERE status = 'ACTIVE'
							`).run();
						}
					}
				}
				message.ack();
			} catch (err) {
				console.error("[Queue Event Processing Error]", err);
				message.retry();
			}
		}
	},
};
