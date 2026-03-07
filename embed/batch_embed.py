import argparse
import psycopg2
from sentence_transformers import SentenceTransformer

DB_URL = "postgresql://agora:agora@localhost:5433/agora"
BATCH_SIZE = 256

def build_text(row):
    method, domain, resource_url, description, networks, assets, price = row
    parts = []
    if method:
        parts.append(method)
    parts.append(resource_url or domain)
    if description:
        parts.append(f"- {description}")
    if networks:
        parts.append(f"Networks: {networks}")
    if assets:
        parts.append(f"Assets: {assets}")
    if price is not None:
        parts.append(f"Price: ${price:.6f}")
    return " ".join(parts)

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--force", action="store_true", help="Re-embed all endpoints, not just nulls")
    args = parser.parse_args()

    model = SentenceTransformer("all-MiniLM-L6-v2")
    conn = psycopg2.connect(DB_URL)

    where = "" if args.force else "WHERE e.embedding IS NULL"
    query = f"""
        SELECT e.id, e.http_method, e.domain, e.resource_url, e.description,
               STRING_AGG(DISTINCT po.network_normalized, ', ') AS networks,
               STRING_AGG(DISTINCT po.asset_name, ', ') AS assets,
               MIN(po.price_usd) AS price
        FROM endpoints e
        LEFT JOIN payment_options po ON po.endpoint_id = e.id
        {where}
        GROUP BY e.id
    """

    cur = conn.cursor()
    cur.execute(query)
    rows = cur.fetchall()
    print(f"Found {len(rows)} endpoints to embed")

    for i in range(0, len(rows), BATCH_SIZE):
        batch = rows[i:i + BATCH_SIZE]
        texts = [build_text(r[1:]) for r in batch]
        embeddings = model.encode(texts)

        for row, emb in zip(batch, embeddings):
            eid = row[0]
            vec = emb.tolist()
            cur.execute("UPDATE endpoints SET embedding = %s::vector WHERE id = %s", (str(vec), eid))

        conn.commit()
        print(f"  Embedded {min(i + BATCH_SIZE, len(rows))}/{len(rows)}")

    cur.close()
    conn.close()
    print("Done!")

if __name__ == "__main__":
    main()
