<script>
  import { branches, pickBranch } from '$lib/branch.svelte.js';
  let { onchange } = $props();
</script>

{#if branches.list.length > 1}
  <select
    aria-label="Sucursal"
    value={branches.id}
    onchange={(e) => (pickBranch(e.currentTarget.value), onchange?.())}
    style="width: auto; margin: 0"
  >
    {#each branches.list as b (b.id)}<option value={b.id}>{b.name}</option>{/each}
  </select>
{/if}
