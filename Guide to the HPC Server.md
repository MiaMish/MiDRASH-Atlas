# Guide to the HPC Server

The shared data on the HPC server is stored under:

```bash
/nas/archive/
```

This directory contains two main kinds of material:

1. **Team workspaces** — separate areas for the Haifa, Paris, Tel Aviv and Bar-Ilan teams.
2. **Shared datasets and resources** — material received from the National Library of Israel (NLI), including the Friedberg Genizah Project (FGP) and several other collections.

The aim of this document is to provide a map of what is currently available and explain how the team folders and permissions should be used.

---

## 1. Understanding the folder listing and permissions

Running:

```bash
ls -la /nas/archive/
```

currently gives:

```text
drwxr-xr-x      6 aladdina jer        FGP
drwxrwxr-x      4 dstoekl  ephe1      Hebrew_Books
drwxr-xr-x 232796 root     root       NLI_GNIZA
drwxrwxr-x 232796 dstoekl  ephe1      NLI_GNIZA_jpgs
drwxr-xr-x  81286 root     root       NLI_MANUSCRIPTS
drwxrwxr-x  81280 dstoekl  ephe1      NLI_MANUSCRIPTS_jpgs
drwxr-xr-x      3 aladdina jer        NLI_catalogue
drwxr-xr-x      3 aladdina jer        NLI_misc
drwxrwxrwx      5 shmidman biu        biu-data
drwxrwxrwx      4 moshel   hai        haifa-data
drwxr-xr-x      8 root     root       media
drwxrwxrwx     25 dstoekl  ephe1      paris-data
drwxrwx---      4 nachum   tau        tau-data
```

There are three pieces of information here that are particularly important for us:

```text
drwxrwxrwx    moshel    hai    haifa-data
^^^^^^^^^^    ^^^^^^    ^^^    ^^^^^^^^^^
permissions   owner     group  directory
```

### Owner and group

Each file or directory has:

- an **owner** — a particular user;
- a **group** — a set of users who belong to the relevant team.

For example:

```text
haifa-data    owner: moshel    group: hai
```

means that the Haifa workspace belongs to the Haifa team. Users who are members of the `hai` group can receive the permissions assigned to that group.

The same principle applies to the other team directories:

```text
biu-data      → Bar-Ilan team
haifa-data    → Haifa team
paris-data    → Paris/EPHE team
tau-data      → Tel Aviv team
```

### Reading the permissions

A string such as:

```text
drwxrwxr-x
```

can be divided into three sets:

```text
rwx | rwx | r-x
 ^      ^     ^
owner  group  everyone else
```

where:

- `r` = **read** — see/read the contents;
- `w` = **write** — create, change or delete material;
- `x` = **access/enter the directory**.

For example:

```text
drwxrwxr-x
```

means:

- the owner can read, write and access;
- members of the group can read, write and access;
- everyone else can read and access, but **cannot modify the contents**.

This distinction is useful because not every dataset should be editable by everyone who can see it.

---

# 2. Team workspaces

The four project teams have dedicated directories under `/nas/archive/`:

```text
biu-data/
haifa-data/
paris-data/
tau-data/
```

These should be treated as the main working spaces for data produced by each team.

For us, the relevant directory is:

```bash
/nas/archive/haifa-data/
```

New data produced by the Haifa team should normally be organized inside this directory.

## Sharing between teams

The team directories are also intended to make it possible to share material between groups.

Permissions can therefore be used differently depending on the dataset.

For example, for a specific use case, we may want a directory in which:

- members of the Haifa team can **read and modify** the data;
- members of the other teams can **read** the data;
- other teams cannot modify it.

In that case the directory should have permissions equivalent to:

```text
owner:  read + write + access
group:  read + write + access
others: read + access
```

or:

```text
drwxrwxr-x
```

If material should remain internal to one team, the permissions can instead be restricted so that other users cannot access it.

It is therefore important when creating or copying new files and folders into `haifa-data/` to check that their permissions correspond to how the material is supposed to be shared.

---

# 3. Shared datasets

Alongside the team workspaces, `/nas/archive/` contains several large shared datasets.

These should generally be treated as **source collections** rather than places for us to add or reorganize our own working files.

## 3.1 National Library of Israel material

The directories beginning with `NLI_` contain material received from the **National Library of Israel**.

The main collections are:

```text
NLI_GNIZA/
NLI_GNIZA_jpgs/

NLI_MANUSCRIPTS/
NLI_MANUSCRIPTS_jpgs/

NLI_catalogue/
NLI_misc/
```

The distinction between the original collections and the `_jpgs` directories is important.

### Genizah

```text
NLI_GNIZA/
```

contains the Genizah data received from the NLI.

```text
NLI_GNIZA_jpgs/
```

contains JPEG versions created by the Daniel Stökl's team from the original NLI files.

### Manuscripts

Likewise:

```text
NLI_MANUSCRIPTS/
```

contains the manuscript data received from the NLI, while:

```text
NLI_MANUSCRIPTS_jpgs/
```

contains JPEG conversions created by the Paris team.

### Why the permissions differ

The original NLI directories are deliberately more restricted than the team workspaces.

For example:

```text
NLI_GNIZA
drwxr-xr-x    root    root
```

means that ordinary users can read/access the collection but cannot alter the original material.

This is desirable: source data received from the NLI should remain stable rather than being modified by individual project teams.

Derived material or work based on those files should therefore normally be placed in the relevant team workspace.

### Paleography material

Additional NLI material is located under:

```bash
NLI_misc/paleography/
```

which currently contains:

```text
AdaYardenitranscription/
Paleography_Hebrew_Script/
```

The first contains the Ada Yardeni material, organized in directories such as:

```text
Ada1/
Ada2/
...
```

with TIFF images inside them.

`Paleography_Hebrew_Script/` contains paleographical image collections organized according to script, region and date, for example:

```text
Ashkenazi_Semi_Cursive-Cursive_1034-1390/
Ashkenazi_Square_1101-1337/
Byzantium_930-1457/
Italian_Semi_Cursive-Cursive_1377-1428/
...
```

These directories likewise contain TIFF image files.

---

## 3.2 Friedberg Genizah Project material

Material originating from the Friedberg Genizah Project is located under:

```bash
/nas/archive/FGP/
```

The current structure is:

```text
FGP/
├── FGP_Geniza_transcriptions/
├── FGP_all_data/
├── FIST_DB_BACKUP/
└── MultiFragmentsWithParts/
```

Two locations are particularly likely to be useful.

### ALTO transcriptions

```bash
FGP/FGP_Geniza_transcriptions/alto/
```

contains the ALTO files.

The hierarchy uses identifiers such as IE, REP and FL. For example:

```text
FGP_Geniza_transcriptions/
└── alto/
    └── IE199040820/
        ├── ie.xml
        └── REP199040862/
            ├── FL199040863_C97973.xml
            ├── FL199040865_C97974.xml
            └── ...
```

### Scan information

```bash
FGP/FGP_all_data/scans/
```

contains:

```text
BavliScans/
BavliScans.xlsx
GenizahScans/
GenizahScans.xlsx
```

`FIST_DB_BACKUP/` is a backup and should not be modified.

---

# 4. Existing resources in the Paris workspace

Daniel Stökl and I (Mia) also mapped the most relevant material for the Haifa team currently available under:

```bash
/nas/archive/paris-data/
```

The Paris team has already created several derived datasets, automatic transcriptions and transformed catalogues that we may be able to reuse.

## 4.1 Automatic transcriptions

According to Daniel, the most important directory is:

```bash
paris-data/automatic_transcriptions/
```

It contains separate material for Genizah fragments and manuscripts.

### Genizah

```bash
paris-data/automatic_transcriptions/geniza/
```

contains several successive processing stages:

```text
all_01/
all_02/
all_03_improved_polys/
all_04_improved_reading_order/
all_04_improved_reading_order_TXTs/
all_05_improved_rdg_order_CSVs/
```

When using this material, the general rule is to use the **highest-numbered `all_XX` version**, since this represents the latest stage of processing.

At present this is:

```bash
paris-data/automatic_transcriptions/geniza/all_05_improved_rdg_order_CSVs/
```

### Manuscripts

Automatic manuscript transcriptions are under:

```bash
paris-data/automatic_transcriptions/manuscripts/
```

The currently available collections include:

```text
BnF/
Haifa_mss/
MidrashHagadol_Roei/
Rashi_Avichai/
TAU_02/
Vatican_mss/
midrashim4haifa/
...
```

The `BnF/` and `Vatican_mss/` collections are the most relevant manuscript material.

For the Haifa team in particular, two directories are also especially relevant:

```text
Haifa_mss/
midrashim4haifa/
```

---

## 4.2 Manuscript images from other sources

Images obtained from sources other than the main NLI/KTIV collections are under:

```bash
paris-data/non_ktiv_mss_imgs/
```

The current directory contains:

```text
BnF/
JTS_Geniza_deskewed/
Vatican_MSS/
more_Friedberg/
```

This is worth checking before independently obtaining manuscript images from these collections.

---

## 4.3 NLI file lists

Because the NLI collections contain extremely large numbers of files, the Paris team created lists that make them easier to search and locate.

These are stored under:

```bash
paris-data/NLI_filelists/
```

These lists can be useful when the goal is to determine whether a particular file or identifier is present without manually navigating through hundreds of thousands of directories.

---

## 4.4 Transformed catalogue data

Processed catalogue files are stored under:

```bash
paris-data/metadata/transformed_catalogues/
```

Current files include:

```text
ALMAS_child2parent_20240207.tsv
GNIZA_catalogue_from_marc_records.tsv
GNIZA_radically_simplified_catalogue_from_marc_records.tsv
GNIZA_simplified_catalogue_from_marc_records.tsv
KTIV_75_fields_2024_02.csv
KTIV_75_fields_2024_02.xlsx
NLI_MANUSCRIPTS_catalogue_from_marcs.tsv
vatican_hebrew_mss.csv
```

These may be useful when working with catalogue metadata or connecting manuscript identifiers across datasets.

---

## 4.5 Other Paris resources

Several other directories may occasionally be useful:

### Shared code

```bash
paris-data/shared_code/
```

contains code shared within the Paris team.

### Models

```bash
paris-data/models/
```

contains copies of models used for tasks such as segmentation.

For models, however, Daniel recommends downloading the relevant current version from **Zenodo** rather than relying on the copies stored on the HPC.

### Meeting recordings

```bash
paris-data/meeting_recordings/
```

contains recordings from earlier project meetings.

---

# 5. Practical use for the Haifa team

In practice, the basic workflow should therefore be:

1. Use `/nas/archive/haifa-data/` for data created or processed by our team.
2. Use the NLI and FGP directories as shared source collections rather than modifying the originals.
3. Use `paris-data/` when relevant. If you encounter a permission problem, contact Daniel Stökl so he can adjust the permissions.
4. Set permissions deliberately when adding material to `haifa-data/`, depending on whether it should be editable or only readable by the other teams.
5. For the Paris automatic Genizah transcriptions, use the latest/highest-numbered `all_XX` version.
6. Avoid moving or renaming existing shared directories, since other workflows may already depend on their current paths.