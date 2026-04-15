// CampusRec - Minimal client-side JavaScript
// No framework; kept minimal for private-network operation.

(function() {
    'use strict';

    // Confirm destructive actions
    document.querySelectorAll('[data-confirm]').forEach(function(el) {
        el.addEventListener('click', function(e) {
            if (!confirm(el.getAttribute('data-confirm'))) {
                e.preventDefault();
            }
        });
    });
})();
