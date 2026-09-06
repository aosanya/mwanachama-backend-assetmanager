# Merchandise absorption mapping

Companion to [architecture.md](architecture.md). Concrete answer to
`mwanachama-backend-api-gateway`'s DEV-1677: how `internal/domain/merchandise`'s
catalogue/ledger/consignment/handout/supplier_delivery map onto this repo's
Asset/Location/Movement/Hold, and whether the held-stock invariant
(`intake − outflows = Σ balances`) survives the mapping. Design only — no code
in this repo changes as a result of this document; DEV-1678 in the gateway is
where the port itself happens.

## 1. Catalogue → Asset

One `Asset` per (Item, Variant) leaf, `TrackingMode = "fungible"` — **not**
one Asset per physical unit. Merchandise's ledger is already keyed at
`(ItemID, VariantID)` everywhere it accounts for stock (`Account`, `Entry`,
`ConsignmentLine`, `Handout`, `SupplierDelivery` all carry both fields), which
is exactly `Asset`'s fungible mode: "one Asset row is a type... the
`merchandise-item`/`merchandise-variant` shape" (architecture.md's own words,
written before this port was scoped). An Item with no variants (a cap)
becomes one Asset; an Item with four sizes (a T-shirt) becomes four.
`PostMovement` only enforces quantity-of-1 for `serialized` Assets — a
fungible Asset's Movement.Quantity is unconstrained, so no structural
mismatch here, provided every merchandise-derived Asset is created fungible.

Gaps: no Item→Variant grouping relation on Asset (§4g below), no
draft/active/retired lifecycle (only binary `Deleted`), no `CreatedBy`, no
generic unique-normalized-label mechanism (merchandise's `NormalizeLabel`
unique index has no analog beyond the hard-coded `SerialTag` partial index).

## 2. Ledger accounts → Location

Merchandise's four `AccountKind`s (`chapter`, `hq`, `in_transit`, `loss`) map
onto `Location.Kind` — but `Location.Kind` is an open, unenforced string with
no Go-level vocabulary the way `IsAccountKind`/`carriesChapter()` enforce for
accounts today; that enforcement would need to be rebuilt at the calling
layer in DEV-1678, or `Location.Kind` gains real validation (open question,
not decided here — architecture.md currently calls `Kind` "descriptive only,"
deliberately).

- **`chapter`/`hq`** — one Location per chapter; `hq` is a chapter-scoped role
  (G248), not a distinct kind of place, same reasoning that already applies
  to accounts.
- **`in_transit`** — one Location per open consignment works, but
  `CreateLocation` is eager/explicit; there is no lazy get-or-create the way
  merchandise's `OpenAccount` provides (G235). DEV-1678's dispatch flow must
  explicitly create the in-transit Location at dispatch time.
- **`loss`** — a single global loss Location is sufficient. Merchandise's
  loss account is per-item, but `GetAssetBalance(assetID, lossLocationID)`
  already reproduces per-item loss tracking without needing a loss account
  per item.
- **Real mismatch, not just a rename**: merchandise's `Account` is one row
  per `(Kind, ChapterID, ItemID, VariantID)` — a chapter gets a *separate*
  account per item. `Location` is per-*place* only; item-scoping lives
  entirely in `GetAssetBalance(assetID, locationID)`. This is actually a
  cleaner shape, but it drops G235's "never traded vs. traded to zero"
  distinction — see §4f.
- **No non-zero-balance delete guard**: merchandise refuses to close an
  account holding units. `DeleteLocation`'s existing guard only checks child
  Locations and the denormalized `Asset.LocationID` field, not a real ledger
  fold — a Location could be soft-deleted while an Asset still nets a
  non-zero balance there via an older Movement. Needs a real guard in
  DEV-1678, not assumed to come free from `DeleteLocation`.

## 3. Ledger entries → Movement

Most fields map directly: `PostedAt`→`CreatedAt`, `OccurredAt`→`OccurredAt`,
`Kind`→`Kind`, `FromAccountID`/`ToAccountID`→`FromLocationID`/`ToLocationID`,
`Quantity`→`Quantity`, `ActorID`→`PerformedBy`, `ReversesEntryID`→
`ReversesMovementID`, `Detail`→`Note`. Three real gaps:

- **No `DocumentKind`/`DocumentID`** — Movement has only a free-text `Note`,
  not the structured pointer back to the causing consignment/handout/
  delivery/write-off/recount row that merchandise's screens trace ledger rows
  through. Needs adding — see §4a.
- **No non-posting "evidence" Movement kind** — merchandise's `EntryCounted`/
  `EntryRecountAsked`/`EntryRecountReplied` (`KindPostsUnits == false`) have
  no analog, and `GetAssetBalance`'s fold has no kind-exclusion mechanism the
  way `KindPostsUnits` provides. This is the single highest-severity gap in
  the whole mapping — see §4b.
- **Sign-convention conflict**: merchandise's `Entry.Quantity` is *always
  positive*; direction is carried by which two accounts are named, never a
  sign (this is explicit and deliberate — a negative write-off is "not even
  expressible" in the current design). `MovementKindAdjusted` explicitly
  *allows* negative `Quantity` for downward corrections — the exact
  convention merchandise rejected. Do not reuse `adjusted`'s sign trick for
  write-off/surplus; see §4c.

No location-scoped Movement query exists today (`MovementFilter` has only
`AssetID`/`Kind`) — needed for per-chapter ledger views and for the
never-traded check in §4f; see §4d.

## 4. Consignment → two Movements, not a Hold

Dispatch is `Movement{Kind: transferred, From: senderChapter, To:
inTransitLocation, Quantity: DispatchedQty}`; receipt is
`Movement{Kind: transferred, From: inTransitLocation, To: receiverChapter,
Quantity: CountedQty}`. This reproduces "short stays in transit" for free,
arithmetically: `GetAssetBalance(asset, inTransitLocation)` after both legs
equals `dispatched − counted`, using the same fold every other balance uses.

**Hold is the wrong tool here, deliberately ruled out.** `Hold` reserves
already-on-hand quantity at *one* location without moving it — a Hold never
changes `GetAssetBalance` until `CommitHold` posts a real Movement. A
dispatched consignment does the opposite: the sender's real balance must drop
*immediately* at dispatch. Modeling dispatch as a Hold would leave dispatched
stock reading as still on the sender's shelf, which is backwards.

`Consignment`/`ConsignmentLine`'s two-count-kept-forever shape stays a
document object one layer above Movement, unchanged — Movement is the
ledger-fold input, not the document, exactly as merchandise already treats
it.

## 5. Handout → `MovementKindDeparted`, member is not a Location

A handout maps onto `MovementKindDeparted` (`From: chapter, To: <empty>`) —
the member does **not** need to become a `Location`. Location's job is to be
a place Movement folds a balance against; a handout is explicitly modeled in
merchandise as a permanent, one-way departure from tracked inventory (M21's
card is a list of view rows, not a balance query) — there is no "member's
shelf" balance to hold anywhere in either system.

Withdrawal maps onto `ReverseMovement`, which already implements "reverse
posts the mirror, original untouched" — the same rule
`rpc_resolve_merchandise_dispute` follows today. One real gap:
`ReverseMovement` has no actor-ownership check, and merchandise's G184 (only
the original issuer may withdraw a handout) has no analog inside it — this
must be enforced by the DEV-1679 handler, consistent with this repo's
existing "no auth of any kind in this package" design.

**Important finding, independent of this mapping**: the gateway's *current*
handout implementation does not post ledger entries at all, despite its own
package doc claiming it does — verified against both
`internal/store/memory/handout_store.go` and
`internal/store/postgres/handout_store.go` (neither calls into
`LedgerRepository`), and there is no HTTP write route for handouts
(`router.go` registers only the member-facing `GET`). This matches
`documentation/2. design/datamodel/handout.md`'s own note that "M47's
capture path and M22's dispute door are not on this plane" — so it is scoped
intentionally, not a hidden regression. It does mean DEV-1678 has no working
reference implementation to port for handout-posts-ledger; it must be
designed fresh from consignment/supplier_delivery's already-real
ledger-posting pattern.

## 6. Supplier delivery → `MovementKindArrived`

Clean match, no synthetic supplier Location. `EntryDeliveredIn` (no
`FromAccountID`, `ToAccountID` required) is exactly `MovementKindArrived`
(`FromLocationID` empty, `ToLocationID` = HQ chapter Location,
`Quantity = CountedQty`, never the delivery-note quantity — merchandise
already posts the count, not the note). The supplier itself is not an object
in either system ("nothing in the set does anything else with one," per
merchandise's own doc) — `SupplierName`/`DeliveryNoteRef` stay free-text
metadata on the surviving `SupplierDelivery` document.

`PostMovement` requires the Asset to already exist — no lazy asset-creation
inside a Movement call. This is not a chicken-and-egg problem as long as
DEV-1678 always creates the catalogue Asset at catalogue-authoring time
(`CreateItem`/`AddVariant`), matching merchandise's own discipline of
creating Items/Variants well before any delivery references them — never
lazily at first-delivery.

G53's two-person delivery approval (`DeliveryPendingApproval` →
`DeliveryPosted`) has no Movement/Asset/Location/Hold analog and needs
none — it's document-level state that calls `PostMovement` exactly once on
approval, the same pattern Consignment already uses.

## 7. Does the held-stock invariant survive?

**Yes, structurally, for the same algebraic reason merchandise's own ledger
closes.** `GetAssetBalance` sums `+Quantity` where `ToLocationID == loc` and
`−Quantity` where `FromLocationID == loc`; `transferred` movements add at one
Location and subtract at another in the same fold pass, and `reversed`
movements are constructed with From/To swapped (cancelling to zero net effect
by construction). Summing balances across every Location for a given Asset
collapses every internal transfer/reversal to zero, leaving:

```
Σ(arrived) − Σ(departed, incl. write-offs once modeled) = Σ balances across all Locations
```

— the same double-entry identity merchandise's ledger already relies on.

**Where the mapping could silently break it, if built carelessly:**

- Reusing `adjusted`'s negative-quantity shortcut for write-off/surplus
  instead of new positive-quantity kinds (§3) — a downward `adjusted` entry
  could arithmetically cancel rather than accumulate against the loss side,
  hiding real shrinkage.
- Adding evidence kinds (`counted`, `recount_asked`, `recount_replied`)
  without also teaching every balance-folding method to exclude them (§3,
  §4b) — exactly the failure `KindPostsUnits`'s own comment warns about:
  "would report the counted figure as stock received, doubling a chapter's
  balance."
- The fungible-vs-serialized quantity check is **confirmed not an issue** —
  `PostMovement` only forces quantity-of-1 for `serialized` Assets, so "50
  units of SKU X move" is natively expressible as long as every
  merchandise-derived Asset is created `fungible`. The risk here is purely
  implementation discipline at Asset-creation time, not a structural gap.
- There is no aggregate/batch balance query — `GetAssetBalance` is strictly
  single-asset/single-location. Verifying `Σ balances` at scale, or an M75-
  style total, needs either a new `AssetManager` method or expensive
  client-side iteration over every known Location × Asset (§4m).

## 4′. Full gap list with proposed resolutions

(Numbered to match the analysis above; "4′" because several gaps surfaced
inside §§2–7 rather than only §4 — kept as one flat list for DEV-1678 to work
through.)

a. **No `DocumentKind`/`DocumentID` on Movement.** Add structured fields so a
   ledger row traces to its causing document, replacing the current
   free-text `Note`.
b. **No non-posting "evidence" Movement kind, and no kind-exclusion in the
   balance fold.** Add evidence kinds and teach every balance-folding method
   to skip them. Highest-severity gap — build this deliberately, not as an
   afterthought.
c. **`adjusted`'s negative-quantity convention conflicts with merchandise's
   always-positive-quantity rule.** Add positive-quantity `written_off`
   (→ loss Location) and `surplus_posted` (arrived-shaped) kinds instead of
   reusing `adjusted`.
d. **No location-scoped Movement query.** Add `FromLocationID`/
   `ToLocationID` (or a combined "either side" filter) to `MovementFilter`
   for per-chapter ledger views and the never-traded check in (f).
e. **No lazy account-opening, no non-zero-balance delete guard.** Callers
   must pre-create every chapter/in-transit Location explicitly, and must
   build their own real-balance check before soft-deleting one.
f. **"Never traded" vs. "traded to zero" indistinguishable** — both read
   balance `== 0`. Resolve via (d): an existence check ("any movement ever
   named this asset+location") rather than a balance check, if the ported UI
   needs this distinction (G235).
g. **No Item→Variant grouping on Asset.** Decide how a catalogue Item's
   several Variant-Assets stay associated — a shared value in
   `Category`/`AttributesJSON`, or accept that `Asset.ID` alone becomes the
   merged key with no "all sizes of this item" query.
h. **No draft/active/retired lifecycle on Asset** (only binary `Deleted`).
   Add one if a draft item must exist without being postable (G336).
i. **No `Asset.CreatedBy`.** Add only if an "Added by" column must survive
   the port — not required for the invariant itself.
j. **No generic unique-normalized-label mechanism** beyond the hard-coded
   `SerialTag` partial index. Must be re-implemented at the calling layer if
   needed.
k. **Handout does not post to the ledger in the gateway today** (§5) — design
   fresh in DEV-1678, including G184's ownership check at the handler layer.
l. **No party-Chapter concept inside this repo at all** — every "chapter"
   becomes an opaque `Location.ID` the gateway must mint and trust; rules
   like "a consignment's two chapters must differ" get zero cross-validation
   here and move entirely into the DEV-1679 handler layer.
m. **No aggregate/batch balance query.** An invariant-verification job or an
   M75-style total needs either a new `AssetManager` method or N×M
   client-side iteration.

## Open questions (judgment calls, not verified facts)

- Whether `Location.Kind` gains Go-level vocabulary enforcement (mirroring
  `IsAccountKind`) is a DEV-1678 choice, not decided here.
- Whether each party Chapter gets exactly one Location row, created at
  chapter-creation time or lazily on first merchandise activity, is
  undecided — no bridging code between the two repos exists yet (expected;
  DEV-1680 hasn't happened).
- Whether the in-transit Location should be parented under the sending
  chapter, the receiving chapter, or left rootless is unresolved —
  merchandise's `in_transit` account carries no `ChapterID` at all, only
  `ConsignmentID`, so there's no existing precedent to copy, and this affects
  `ListDescendantLocations`-based aggregate queries.
- Whether `Consignment`/`SupplierDelivery`'s "three writes as one act"
  transactional guarantee survives `PostMovement` + the denormalized
  `Asset.LocationID` sync being two separate, non-atomic calls (a gap this
  repo's own `movement.go` already documents for ordinary movements) needs a
  specific look in DEV-1678 — not confirmed one way or the other here.
