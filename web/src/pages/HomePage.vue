<template>
  <q-page padding>
    <div class="row items-end q-mb-md">
      <div class="col">
        <div class="text-h4 q-mb-xs">mini-acm</div>
        <p class="text-body2 text-grey-7 q-mb-none home-lead">
          One RHEL host. Libvirt guests. A VyOS edge. Mgmt SNO runs ACM; deployment clusters are managed on the lab LAN.
        </p>
      </div>
      <q-btn color="primary" unelevated label="Open MockUps" icon="account_tree"
        :to="{ name: 'mockups' }" />
    </div>

    <ArchitectureBoxView class="q-mb-lg" />

    <div class="home-steps q-mb-lg">
      <button type="button" class="home-step" @click="$router.push({ name: 'mockups' })">
        <span class="home-step-n">1</span>
        <span>
          <span class="home-step-t">MockUp</span>
          <span class="home-step-d">Create a lab rack blueprint</span>
        </span>
      </button>
      <button type="button" class="home-step" @click="$router.push({ name: 'mockups' })">
        <span class="home-step-n">2</span>
        <span>
          <span class="home-step-t">Topology</span>
          <span class="home-step-d">Place host, gateway, mgmt &amp; deployments</span>
        </span>
      </button>
      <button type="button" class="home-step" @click="$router.push({ name: 'mockups' })">
        <span class="home-step-n">3</span>
        <span>
          <span class="home-step-t">Wizard</span>
          <span class="home-step-d">Fill gaps · derive YAML · run CLI</span>
        </span>
      </button>
    </div>

    <p class="text-caption text-grey-6">
      LAB / TEST / DEV ONLY — secrets stay in env / file paths, never in MockUp YAML in git.
    </p>
  </q-page>
</template>

<script setup>
import ArchitectureBoxView from 'src/components/ArchitectureBoxView.vue'
</script>

<style scoped>
.home-lead {
  max-width: 42rem;
  line-height: 1.45;
}
.home-steps {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 0.75rem;
}
@media (max-width: 700px) {
  .home-steps { grid-template-columns: 1fr; }
}
.home-step {
  display: flex;
  align-items: flex-start;
  gap: 0.75rem;
  text-align: left;
  border: 1px solid #e0e0e0;
  border-radius: 8px;
  background: #fff;
  padding: 0.85rem 1rem;
  cursor: pointer;
}
.home-step:hover {
  border-color: #90caf9;
  background: #f7fbff;
}
.home-step-n {
  width: 1.5rem;
  height: 1.5rem;
  border-radius: 50%;
  background: #e3f2fd;
  color: #0d47a1;
  font-size: 0.8rem;
  font-weight: 700;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.home-step-t {
  display: block;
  font-weight: 700;
  color: #263238;
  font-size: 0.95rem;
}
.home-step-d {
  display: block;
  margin-top: 0.15rem;
  color: #78909c;
  font-size: 0.8rem;
  line-height: 1.3;
}
</style>
