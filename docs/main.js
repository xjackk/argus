// Scroll reveal. The .rv hidden state is gated on the `js` class set in <head>,
// so a no-JS visitor still sees the full page.
(function(){
  var els = document.querySelectorAll('.rv');
  if(!('IntersectionObserver' in window) || window.matchMedia('(prefers-reduced-motion: reduce)').matches){
    els.forEach(function(el){ el.classList.add('in'); });
    return;
  }
  var io = new IntersectionObserver(function(entries){
    entries.forEach(function(e){
      if(e.isIntersecting){ e.target.classList.add('in'); io.unobserve(e.target); }
    });
  }, { rootMargin:'0px 0px -8% 0px', threshold:0.06 });

  var vh = window.innerHeight;
  els.forEach(function(el, i){
    // Anything already on screen at load paints immediately — no fade, no
    // delay. A visitor should never land on a blank hero waiting for JS.
    if(el.getBoundingClientRect().top < vh){
      el.style.transition = 'none';
      el.classList.add('in');
      return;
    }
    el.style.transitionDelay = Math.min(i % 5, 4) * 55 + 'ms';
    io.observe(el);
  });
})();
