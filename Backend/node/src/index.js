import http from 'node:http';
import pg from 'pg';

const pool = new pg.Pool({ connectionString: process.env.DATABASE_URL });

http
  .createServer(async (req, res) => {
    if (req.url === '/health') {
      try {
        await pool.query('SELECT 1');
        res.end('ok');
      } catch {
        res.writeHead(503).end('db unavailable');
      }
      return;
    }
    res.writeHead(404).end();
  })
  .listen(process.env.NODE_API_PORT || 3000);
