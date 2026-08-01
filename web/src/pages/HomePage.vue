<template>
  <q-page class="home-page">
    <section class="home-hero">
      <div class="home-hero-copy">
        <p class="home-kicker">Lab MockUp canvases</p>
        <h1 class="home-title">mock-me</h1>
        <p class="home-lead">
          Build a product blueprint by genre, validate the plan, then deploy against a ready MACHINE-HOST.
          Wizard only collects gaps — Topology paints the rack.
        </p>
        <div class="home-hero-actions">
          <q-btn color="primary" unelevated icon="add" label="New MockUp" @click="openCreate()" />
          <q-btn flat color="primary" label="Browse MockUps" :to="{ name: 'mockups' }" />
          <q-btn flat color="primary" label="Inventory" :to="{ name: 'inventory' }" />
        </div>
      </div>
    </section>

    <section class="home-section">
      <div class="row items-center q-mb-sm">
        <div>
          <div class="section-title">Supported styles</div>
          <div class="section-sub">Carousel of catalog offerings — available styles first; stubs show as coming soon.</div>
        </div>
        <q-space />
        <q-btn flat dense round icon="chevron_left" :disable="slide === 0" @click="slide = Math.max(0, slide - 1)" />
        <q-btn flat dense round icon="chevron_right" :disable="slide >= slides.length - 1" @click="slide = Math.min(slides.length - 1, slide + 1)" />
      </div>

      <q-carousel
        v-model="slide"
        animated
        swipeable
        control-color="primary"
        class="style-carousel"
        height="280px"
        :navigation="slides.length > 1"
      >
        <q-carousel-slide v-for="(s, i) in slides" :key="s.id" :name="i" class="style-slide">
          <div class="style-panel" :class="{ soon: !s.available }">
            <div class="style-meta">
              <q-badge :color="s.available ? 'positive' : 'grey-6'" outline>
                {{ s.available ? 'Available' : 'Coming soon' }}
              </q-badge>
              <div class="style-genre">{{ s.genreLabel }}</div>
              <div class="style-name">{{ s.label }}</div>
              <p class="style-desc">{{ s.description }}</p>
              <q-btn
                v-if="s.available"
                color="primary"
                outline
                dense
                label="New from this style"
                class="q-mt-sm"
                @click="openCreate(s)"
              />
            </div>
            <div class="style-visual">
              <ArchitectureBoxView v-if="s.id === 'acm-multi-cluster'" compact />
              <div v-else class="style-placeholder">
                <q-icon :name="s.icon" size="48px" color="blue-grey-4" />
                <span>{{ s.available ? 'Preview later' : 'Catalog stub' }}</span>
              </div>
            </div>
          </div>
        </q-carousel-slide>
      </q-carousel>
    </section>

    <section class="home-section">
      <div class="section-title">What each surface does</div>
      <div class="section-sub q-mb-md">Home is the launch pad — these three are where work happens after you create a MockUp.</div>
      <div class="role-grid">
        <article class="role-card">
          <div class="role-icon"><q-icon name="account_tree" /></div>
          <h3>Topology</h3>
          <p>Canvas of the rack — MACHINE-HOST, VyOS, OCP-MGMT, ACM, deployments. Edit objects, Validate, Deploy.</p>
          <q-btn flat dense color="primary" label="Open MockUps" :to="{ name: 'mockups' }" />
        </article>
        <article class="role-card">
          <div class="role-icon"><q-icon name="playlist_play" /></div>
          <h3>Wizard</h3>
          <p>Data collector only — pull secret, SSH key, channels, ISO paths. Does not invent topology; then Derive YAML for CLI.</p>
          <q-btn flat dense color="primary" label="New MockUp → Wizard" @click="openCreate(null, 'wizard')" />
        </article>
        <article class="role-card">
          <div class="role-icon"><q-icon name="dns" /></div>
          <h3>Inventory</h3>
          <p>Real MACHINE-HOST targets. Probe red/yellow/green; Stretched uses VPN when the cluster cannot reach LAN.</p>
          <q-btn flat dense color="primary" label="Open Inventory" :to="{ name: 'inventory' }" />
        </article>
      </div>
    </section>

    <section class="home-section home-lifecycle">
      <div class="section-title">Click-through lifecycle</div>
      <div class="life-row">
        <span class="life-pill grey">Created</span>
        <q-icon name="arrow_forward" size="16px" color="grey-6" />
        <span class="life-pill blue">Configured</span>
        <q-icon name="arrow_forward" size="16px" color="grey-6" />
        <span class="life-pill purple">Validated</span>
        <q-icon name="arrow_forward" size="16px" color="grey-6" />
        <span class="life-pill orange">Deploying</span>
        <q-icon name="arrow_forward" size="16px" color="grey-6" />
        <span class="life-pill green">Deployed</span>
      </div>
      <p class="text-caption text-grey-6 q-mt-sm q-mb-none">
        Defaults path → Configured after derive. Validate &amp; Deploy live on the MockUp card (needs a green Inventory host).
      </p>
    </section>

    <CreateMockUpDialog
      v-model="createOpen"
      :catalog="catalog"
      :prefer-genre="preferGenre"
      :prefer-style="preferStyle"
      :prefer-path="preferPath"
      @created="onCreated"
    />
  </q-page>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { getCatalog } from 'src/services/api'
import ArchitectureBoxView from 'src/components/ArchitectureBoxView.vue'
import CreateMockUpDialog from 'src/components/CreateMockUpDialog.vue'

const catalog = ref({ genres: [], styles: [] })
const slide = ref(0)
const createOpen = ref(false)
const preferGenre = ref('')
const preferStyle = ref('')
const preferPath = ref('')

const genreLabel = (id) =>
  (catalog.value.genres || []).find((g) => g.id === id)?.label || id

const iconFor = (styleId) => {
  const map = {
    'acm-multi-cluster': 'hub',
    'single-sno-ocp': 'memory',
    'windows-ui': 'desktop_windows',
    'web-full-stack': 'language',
    'infra-node-network-payload': 'lan',
    'surfing-cdn-r2': 'cloud',
    'cdn-mgr-realm': 'domain',
  }
  return map[styleId] || 'category'
}

const slides = computed(() => {
  const styles = [...(catalog.value.styles || [])]
  styles.sort((a, b) => Number(b.available) - Number(a.available))
  return styles.map((s) => ({
    ...s,
    genreLabel: genreLabel(s.genre),
    icon: iconFor(s.id),
  }))
})

function openCreate(style, path) {
  preferGenre.value = style?.genre || ''
  preferStyle.value = style?.id || ''
  preferPath.value = path || ''
  createOpen.value = true
}

function onCreated() {
  // dialog navigates; no-op hook for future refresh
}

onMounted(async () => {
  try {
    catalog.value = await getCatalog()
  } catch {
    catalog.value = { genres: [], styles: [] }
  }
})
</script>

<style scoped>
.home-page {
  padding: 1.25rem 1.5rem 2rem;
  max-width: 1100px;
  margin: 0 auto;
  background:
    radial-gradient(ellipse 80% 50% at 10% -10%, rgba(21, 101, 192, 0.08), transparent 55%),
    radial-gradient(ellipse 60% 40% at 100% 0%, rgba(0, 131, 143, 0.06), transparent 50%),
    #f5f7fa;
}
.home-hero {
  margin-bottom: 1.5rem;
}
.home-kicker {
  margin: 0 0 0.35rem;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  font-size: 0.72rem;
  font-weight: 700;
  color: #546e7a;
}
.home-title {
  margin: 0 0 0.5rem;
  font-size: clamp(1.8rem, 3vw, 2.35rem);
  font-weight: 800;
  letter-spacing: -0.02em;
  color: #0d47a1;
}
.home-lead {
  margin: 0 0 1rem;
  max-width: 38rem;
  color: #546e7a;
  line-height: 1.45;
  font-size: 0.95rem;
}
.home-hero-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
  align-items: center;
}
.home-section {
  margin-bottom: 1.75rem;
}
.section-title {
  font-weight: 750;
  font-size: 1.05rem;
  color: #263238;
}
.section-sub {
  color: #78909c;
  font-size: 0.82rem;
  line-height: 1.35;
  max-width: 40rem;
}
.style-carousel {
  border-radius: 14px;
  background: #fff;
  border: 1px solid #e0e6ed;
  box-shadow: 0 8px 24px rgba(38, 50, 56, 0.06);
}
.style-slide {
  padding: 0.85rem 1rem;
}
.style-panel {
  display: grid;
  grid-template-columns: minmax(200px, 0.9fr) 1.2fr;
  gap: 1rem;
  height: 100%;
  align-items: stretch;
}
.style-panel.soon .style-visual {
  opacity: 0.72;
  filter: grayscale(0.25);
}
.style-genre {
  margin-top: 0.55rem;
  font-size: 0.72rem;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: #78909c;
  font-weight: 700;
}
.style-name {
  font-size: 1.15rem;
  font-weight: 750;
  color: #102a43;
  margin-top: 0.15rem;
}
.style-desc {
  margin: 0.45rem 0 0;
  color: #607d8b;
  font-size: 0.82rem;
  line-height: 1.4;
  display: -webkit-box;
  -webkit-line-clamp: 4;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.style-visual {
  min-height: 0;
  border-radius: 10px;
  overflow: hidden;
  background: #f8fafc;
  border: 1px solid #eceff1;
  display: flex;
  align-items: center;
  justify-content: center;
}
.style-placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.4rem;
  color: #90a4ae;
  font-size: 0.8rem;
}
.role-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 0.75rem;
}
.role-card {
  background: #fff;
  border: 1px solid #e0e6ed;
  border-radius: 12px;
  padding: 1rem;
}
.role-icon {
  width: 2rem;
  height: 2rem;
  border-radius: 8px;
  background: #e3f2fd;
  color: #1565c0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 0.55rem;
}
.role-card h3 {
  margin: 0 0 0.35rem;
  font-size: 0.98rem;
  color: #263238;
}
.role-card p {
  margin: 0 0 0.55rem;
  color: #607d8b;
  font-size: 0.8rem;
  line-height: 1.4;
  min-height: 3.6em;
}
.home-lifecycle {
  padding: 1rem 1.1rem;
  background: #fff;
  border-radius: 12px;
  border: 1px solid #e0e6ed;
}
.life-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.35rem;
  margin-top: 0.55rem;
}
.life-pill {
  font-size: 0.72rem;
  font-weight: 700;
  padding: 0.25rem 0.55rem;
  border-radius: 999px;
  color: #fff;
}
.life-pill.grey { background: #78909c; }
.life-pill.blue { background: #1976d2; }
.life-pill.purple { background: #5e35b1; }
.life-pill.orange { background: #ef6c00; }
.life-pill.green { background: #2e7d32; }

@media (max-width: 800px) {
  .style-panel { grid-template-columns: 1fr; }
  .role-grid { grid-template-columns: 1fr; }
  .style-carousel { height: 420px !important; }
}
</style>
