---
title: Home
nav_order: 1
description: "Keramos is a Kubernetes package manager built around expression-based templating, layered composition, and dependency-aware orchestration."
---

<div class="keramos-hero">
  <p class="keramos-hero__eyebrow">Kubernetes packaging, reimagined</p>
  <h1>Keramos</h1>
  <p class="lead">A Kubernetes package manager built around <strong>expression-based templating</strong>, <strong>layered composition</strong>, and <strong>dependency-aware orchestration</strong> — with a plan/diff/drift workflow you can actually trust.</p>
  <a href="{{ '/guides/quickstart.html' | relative_url }}" class="btn btn-hero">Get started →</a>
  <a href="{{ '/cli/README.html' | relative_url }}" class="btn btn-ghost">CLI reference</a>
  <a href="https://github.com/ebogdum/keramos" class="btn btn-ghost">GitHub</a>
</div>

<div class="keramos-terminal">
  <div class="keramos-terminal__dots"><span></span><span></span><span></span></div>
  <pre><span class="c-prompt">$</span> <span class="c-cmd">keramos install web ./web --values prod.yaml</span>
<span class="c-out">Planned 14 resources · 0 drift · 0 warnings</span>
<span class="c-ok">✔ release "web" installed (revision 1)</span>

<span class="c-prompt">$</span> <span class="c-cmd">keramos diff web ./web --values prod.yaml</span>
<span class="c-out">~ Deployment/web            replicas 3 → 5</span>
<span class="c-out">~ ConfigMap/web-config      data.LOG_LEVEL info → debug</span>
<span class="c-ok">2 changes, 12 unchanged</span></pre>
</div>

## Start here

New to Keramos? The **Quickstart** takes you from an empty directory to a deployed, upgraded, and rolled-back release in a few minutes.

<div class="keramos-cards">
  <a class="keramos-card" href="{{ '/guides/quickstart.html' | relative_url }}">
    <div class="keramos-card__icon">🚀</div>
    <div class="keramos-card__title">Quickstart</div>
    <div class="keramos-card__desc">Install, upgrade, and roll back your first release.</div>
  </a>
  <a class="keramos-card" href="{{ '/guides/packages.html' | relative_url }}">
    <div class="keramos-card__icon">📦</div>
    <div class="keramos-card__title">Packages</div>
    <div class="keramos-card__desc">Structure a Keramos package and its templates.</div>
  </a>
  <a class="keramos-card" href="{{ '/guides/values.html' | relative_url }}">
    <div class="keramos-card__icon">🎛️</div>
    <div class="keramos-card__title">Values</div>
    <div class="keramos-card__desc">Layer and override configuration cleanly.</div>
  </a>
  <a class="keramos-card" href="{{ '/guides/migration.html' | relative_url }}">
    <div class="keramos-card__icon">🔀</div>
    <div class="keramos-card__title">Migrate from Helm</div>
    <div class="keramos-card__desc">Convert an existing Helm chart to a Keramos package.</div>
  </a>
</div>

## Explore the docs

<div class="keramos-cards">
  <a class="keramos-card" href="{{ '/guides/index.html' | relative_url }}">
    <div class="keramos-card__icon">📘</div>
    <div class="keramos-card__title">Guides</div>
    <div class="keramos-card__desc">Task-focused walkthroughs for real work.</div>
  </a>
  <a class="keramos-card" href="{{ '/cli/README.html' | relative_url }}">
    <div class="keramos-card__icon">⌨️</div>
    <div class="keramos-card__title">CLI reference</div>
    <div class="keramos-card__desc">Every command, flag, and option — with worked input→output examples.</div>
  </a>
  <a class="keramos-card" href="{{ '/templates/index.html' | relative_url }}">
    <div class="keramos-card__icon">🧩</div>
    <div class="keramos-card__title">Templates</div>
    <div class="keramos-card__desc">The <code>${...}</code> expression language, control flow, and ~200 functions.</div>
  </a>
  <a class="keramos-card" href="{{ '/reference/index.html' | relative_url }}">
    <div class="keramos-card__icon">📑</div>
    <div class="keramos-card__title">Reference</div>
    <div class="keramos-card__desc">Manifest and values files, field by field.</div>
  </a>
  <a class="keramos-card" href="{{ '/comparison.html' | relative_url }}">
    <div class="keramos-card__icon">⚖️</div>
    <div class="keramos-card__title">Comparison</div>
    <div class="keramos-card__desc">Keramos vs Helm, Kustomize, kapp, and kpt.</div>
  </a>
  <a class="keramos-card" href="{{ '/faq.html' | relative_url }}">
    <div class="keramos-card__icon">❓</div>
    <div class="keramos-card__title">FAQ</div>
    <div class="keramos-card__desc">Frequent questions, answered.</div>
  </a>
</div>

## How these docs are written

Every reference example shows **input → output**: the file you write or the command you run, and the exact result it produces. When a command reads hidden state — a stored release, the live cluster — the docs show that state and trace each output line back to its cause. See [`keramos drift`]({{ '/cli/drift.html' | relative_url }}) for the fullest example of this style.
