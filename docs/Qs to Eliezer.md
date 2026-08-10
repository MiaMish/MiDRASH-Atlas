# Questions to Eliezer

These are unresolved scholarly/data-model questions discovered while joining
the replacement Midrash Atlas sources. The raw source files have not been
changed. Temporary decisions and stub records are explicitly labelled so they
can be replaced after review.

## 1. Hiburim named only by manuscript contents records

The manuscript CSV uses the following eight contents labels, but none matches
an ontology name or alias. We created temporary hibur records with IDs beginning
`stub:mss:` so the manuscript evidence is not lost.

For each label, please tell us whether it is:

1. an independent hibur that should receive a permanent ontology ID;
2. an alias of an existing hibur (and, if so, which one);
3. a category or corpus rather than a hibur; or
4. an unidentified description that should remain attached only to the item.

### אנתולוגיות לא מוכרות

- 31 manuscript references.
- Manuscript CSV rows: 28, 29, 30, 33, 65, 66, 67, 68, 74, 75, 76, 77,
  78, 80, 82, 98, 99, 107, 155, 209, 234, 301, 302, 304, 306, 308, 310,
  312, 330, 335, 336.
- Question: is this intentionally a placeholder for unidentified contents
  rather than a hibur?
- Data-quality note: row 306 appears shifted or malformed; its `מספר מערכת`
  cell contains work labels instead of a system ID.

### ברייתא דר' אלעזר בן יוסי הגלילי

- 11 manuscript references.
- Manuscript CSV rows: 67, 324, 325, 326, 327, 328, 329, 330, 331, 332, 334.

### בריתא דמלאכת המשכן

- 5 manuscript references.
- Manuscript CSV rows: 84, 313, 314, 315, 316.

### מדרש השכם (והזהיר)

- 1 manuscript reference.
- Manuscript CSV row: 319; system ID `990001278560205171`.

### מדרש פנים אחרת נוסח ב (אסתר)

- 1 manuscript reference.
- Manuscript CSV row: 201; system ID `990000573820205171`.
- Question: is this simply a spelling variant of ontology work `2:3:2.0`,
  whose preferred name is `מדרש פנים אחרות ב`?

### מדרשי ר' שמואל מסנות

- 6 manuscript references.
- Manuscript CSV rows: 295, 296, 297, 298, 299, 300.
- Question: does this plural label identify one work, multiple works, or a
  corpus/category?

### מסכתות קטנות

- 10 manuscript references.
- Manuscript CSV rows: 192, 201, 202, 308, 314, 315, 330, 335, 336, 343.
- Question: should this remain a collective corpus, or should the manuscripts
  be connected to individual minor tractates?

### תלמוד תורה

- 6 manuscript references.
- Manuscript CSV rows: 37, 79, 203, 204, 205, 206.

The complete system-ID lists and explanatory comments are stored beside each
temporary record in
`data/sources_new/hiburim_from_manuscript_references.csv`.

## 2. ספרי אסופות של מדרשים: fourteen edition rows to classify

The project decision is settled: the hibur ontology is authoritative. It has
two separate canonical works:

- In the hibur ontology, row 156 is `אוצר המדרשים` (`3:18:1.0`) and row 157
  is `בתי מדרשות` (`3:19:1.0`). Both rows give `ספרי אסופות של מדרשים` as
  their Reizel name and both give Reizel ID `54`.
- In the creation CSV, row 41 uses `ספרי אסופות של מדרשים` as an umbrella
  title. Its alternate names and bibliography enumerate many distinct
  anthologies, including בית המדרש, בתי מדרשות, אוצר מדרשים, גנזי שכטר, and
  others.
- Reizel's shared label and ID are preserved as evidence that this interpreter
  merged the works; they do not merge the two project ontology entities.

The editions CSV contains these fourteen rows under Reizel's umbrella label:

| Edition row | Publication description | Place | Year |
| ---: | --- | --- | ---: |
| 218 | A. Jellinek, *Bet HaMidrash* I–VI | Leipzig | 1853 |
| 219 | H. M. Horovitz, *Aggadat Aggadot* | Berlin | 1881 |
| 220 | H. M. Horovitz, *Bet Eked HaAggadot* | Frankfurt | 1881 |
| 221 | H. M. Horovitz, *Tosefta Atikta* | Frankfurt | 1890 |
| 222 | S. A. Wertheimer, *Batei Midrashot* | Jerusalem | 1893 |
| 223 | S. A. Wertheimer, *Sefer Leket Midrashim* | Jerusalem | 1903 |
| 224 | S. A. Wertheimer, *Sefer Otzar Midrashim Kitvei Yad* | Jerusalem | 1913 |
| 225 | S. A. Wertheimer, *Sefer Midrashim Kitvei Yad* | Jerusalem | 1923 |
| 226 | A. Grünhut, *Sefer HaLikutim* | Jerusalem | 1898 |
| 227 | J. D. Eisenstein, *Otzar Midrashim* | New York | 1915 |
| 228 | L. Ginzberg, *Ginzei Schechter* I | New York | 1928 |
| 229 | A. Deshe-Halevi, *Sefer Yalkut Midrashim* | Safed | 2003 |
| 230 | Mann, *The Bible as Read and Preached in the Old Synagogue* | Cincinnati | 1940 |
| 317 | J. D. Eisenstein, *Otzar Midrashim* | New York | 1915 |

Question: for each of these fourteen edition rows, should it embody
`אוצר המדרשים` (`3:18:1.0`), `בתי מדרשות` (`3:19:1.0`), another specific
hibur, or remain only a publication of a broader anthology/corpus? Rows 227 and
317 appear to describe the same Eisenstein publication; should row 317 be
treated as its reprint/duplicate assertion?

## 3. Generated temporary IDs for early ontology rows

The following ontology entries have no `ID חדש?`: Mekhilta de-Rabbi Ishmael,
Mekhilta de-Rabbi Shimon bar Yohai, Sifra, Sifrei, Sifrei Numbers, Sifrei
Deuteronomy, Sifrei Zuta, Sifrei Zuta to Numbers, Sifrei Zuta to Deuteronomy,
and Mekhilta to Deuteronomy / Midrash Tannaim. We generated explicitly
temporary `stub:ontology:*` IDs without modifying the raw ontology CSV.

None of these ten rows is used by the manuscript CSV. Exact creation and
edition references are listed below.

| Ontology row and entry | Generated temporary ID | MSS rows | Creation rows | Edition rows |
| --- | --- | --- | --- | --- |
| 2 — Mekhilta de-Rabbi Ishmael | `stub:ontology:mekhilta-rabbi-ishmael` | none | 3 | 2, 3, 73–84 |
| 3 — Mekhilta de-Rabbi Shimon bar Yohai | `stub:ontology:mekhilta-rabbi-shimon-bar-yohai` | none | 4 | 4, 85–88 |
| 4 — Sifra | `stub:ontology:sifra` | none | 5 | 5, 89–98 |
| 6 — Sifrei | `stub:ontology:sifrei` | none | none | none |
| 7 — Sifrei Numbers | `stub:ontology:sifrei-numbers` | none | 6 | 6, 7, 99–107 |
| 8 — Sifrei Zuta | `stub:ontology:sifrei-zuta` | none | none | none |
| 9 — Sifrei Zuta to Numbers | `stub:ontology:sifrei-zuta-numbers` | none | 7 | 108–112 |
| 10 — Sifrei Deuteronomy | `stub:ontology:sifrei-deuteronomy` | none | 8 | 8, 113–123 |
| 11 — Mekhilta to Deuteronomy / Midrash Tannaim | `stub:ontology:mekhilta-deuteronomy` | none | 9 | 9, 124 |
| 12 — Sifrei Zuta to Deuteronomy | `stub:ontology:sifrei-zuta-deuteronomy` | none | 10 | 10, 125 |

Question: what permanent `ID חדש?` should replace each generated ID? Should
the unused generic `Sifrei` and `Sifrei Zuta` rows be hierarchical parents
rather than ordinary hiburim?

Two additional ontology rows remain as-is and are excluded from canonical
export because no other new source uses them:

- `Mekhilta Leviticus`, whose Wikipedia-side marker says `<חיבור משוער>`
  (hypothetical/reconstructed work). It is not referenced by the other three
  new CSVs.
- `אגדות התלמוד`, present only in the Wikipedia column with no English title,
  transliterated title, or join key. It is not referenced by the other three
  new CSVs.

No ID was generated for these two unused rows. They can be reconsidered if a
future source actually references them.
