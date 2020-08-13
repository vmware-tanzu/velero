(function () {

  $('.match-height').matchHeight();

  $('#nav-mobile-toggle').click(function () {
    $('#header-nav').toggleClass('on');
  });

  $('ul#header-nav li.' + $('body').attr('id')).addClass('selected');
})();