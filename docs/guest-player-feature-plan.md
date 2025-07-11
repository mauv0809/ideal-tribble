# Guest Player & New Member Feature Plan

This document outlines the design for handling guest players and adding new club members to the Wally application in a robust and automated way.

## Guiding Principles

1.  **Authoritative Source of Truth:** The official company Slack channel is the single source of truth for who is considered a "club member".
2.  **Data Cleanliness:** The production `players` table should only ever contain confirmed club members. It should not be polluted with one-off "guest" opponents.
3.  **Unambiguous Statistics:** Only matches where all four participants are confirmed, linked club members should be included in leaderboards and statistics.
4.  **Automated Discovery:** The system should automatically discover and link the Playtomic identity of new members once they are in the Slack channel and play a match.

---

## Proposed Architecture: "Slack-Vetted Discovery"

This approach uses Slack as the definitive roster and uses match data as the mechanism to discover a new member's Playtomic ID.

### 1. New Data Structures

#### `players` Table Modification

We will add a `status` column and redefine the keys in the `players` table.

- **`slack_id` (TEXT, PRIMARY KEY):** This will be the new primary key. It's unique and always present for a member added from Slack.
- **`playtomic_id` (TEXT, UNIQUE, NULLABLE):** The Playtomic ID will be nullable to support the `'unlinked'` status. A `UNIQUE` constraint is critical to ensure it can be reliably referenced by other tables.
- **`status` (string):** Can be one of two values:
  - `'unlinked'`: Represents a person who is in the Slack channel, but whose Playtomic ID has not yet been discovered. This is a potential member.
  - `'linked'`: A confirmed club member whose Slack and Playtomic identities have been successfully associated.
- **`name` (TEXT):** The user's name, likely from their Slack profile.

#### Foreign Key Considerations in Other Tables

This schema change impacts how other tables reference players.

- **`player_stats` table:** This table should **continue to use `playtomic_id`** as its foreign key (`player_playtomic_id` that references `players.playtomic_id`).
  - **Rationale:** The stats are generated from game data where players are identified by `playtomic_id`. Using it as the key makes the stat update logic more direct and efficient, as no extra lookup is needed to find a different ID.

### 2. The Population and Linking Workflow

The process is divided into two parts: populating potential members from Slack, and then linking them via match processing.

#### Step A: Populate Unlinked Members from Slack

- A new, recurring job (or an enhancement to an existing one) will be created.
- This job's responsibility is to fetch the full member list from the designated Slack channel.
- It will iterate through the list and for each Slack user, it will `UPSERT` a record into our `players` table based on `slack_id`.
- The record will be created with:
  - `slack_id`: The user's Slack ID.
  - `name`: The user's full name from their Slack profile.
  - `status`: `'unlinked'`.
  - `playtomic_id`: `NULL`.

This ensures our database always has an up-to-date list of potential members waiting to be linked.

#### Step B: The Match Processor's Linking Logic

The `ProcessMatchesHandler` will be updated with the following logic for every match it ingests:

1.  **Check Players:** For each of the 4 players in the match data from Playtomic, check their status in our `players` table using their `playtomic_id`. Count how many are already `'linked'`.

2.  **The 4/4 Case (All Members):**

    - If all 4 players are found and have a status of `'linked'`, the match is a valid "Club Match".
    - Process stats and update leaderboards as normal.

3.  **The 3/4 Case (New Member Discovery):**

    - If exactly 3 players are `'linked'`, this triggers the discovery process for the 4th, unknown player.
    - Take the name string of this 4th player from the Playtomic data.
    - Perform a **fuzzy string comparison** against the `name` column of all players in our database who have the status `'unlinked'`.
    - **If a confident match is found** (e.g., Levenshtein distance > 0.9):
      - Update the matched `'unlinked'` player's record.
      - Set their `playtomic_id` to the ID from the match.
      - Change their `status` to `'linked'`.
      - The match can now be immediately re-evaluated as a 4/4 "Club Match" and processed for stats in the same run.
      - Optionally, send a confirmation to a Slack channel: "✅ Successfully linked @[SlackName] to Playtomic player '[PlaytomicName]'!"

4.  **All Other Cases (<3 Members or No Confident Match):**
    - If a confident name match for the 4th player cannot be found, or if the match has fewer than 3 linked members.
    - The match is **ignored**. It is not a club match. No data is stored, no stats are calculated. The unknown player is treated as a true guest and is not added to our database.

### 3. Edge Case Management

- **Manual Override Command:** For cases where fuzzy name matching fails (e.g., "Jonny" vs "Jonathan"), an admin-only Slack command will be created: `/padel-link-member [slack_user_mention] [playtomic_id]`. This command will manually perform the link, setting the `playtomic_id` and updating the status to `'linked'`. This provides a necessary escape hatch for automation failures.

This design provides a robust, automated, and maintainable system for managing club membership and ensuring the integrity of our player statistics.
